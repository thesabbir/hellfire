package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/thesabbir/hellfire/pkg/audit"
	"github.com/thesabbir/hellfire/pkg/auth"
	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/config"
	"github.com/thesabbir/hellfire/pkg/db"
	apierrors "github.com/thesabbir/hellfire/pkg/errors"
	"github.com/thesabbir/hellfire/pkg/handlers"
	"github.com/thesabbir/hellfire/pkg/hfconfig"
	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/middleware"
	"github.com/thesabbir/hellfire/pkg/uci"
)

// @title Hellfire API
// @version 1.0
// @description UCI-like configuration management system for Debian routers
// @termsOfService http://swagger.io/terms/

// @contact.name Sabbir Ahmed
// @contact.url https://github.com/thesabbir/hellfire

// @license.name GPL-3.0
// @license.url https://www.gnu.org/licenses/gpl-3.0.html

// @host localhost:8888
// @BasePath /api
// @schemes http https

func startAPIServer(port int, manager *config.Manager) error {
	// Load Hellfire configuration
	hfConfig, err := hfconfig.Load("")
	if err != nil {
		logger.Warn("Failed to load Hellfire config, using defaults", "error", err)
		hfConfig = hfconfig.DefaultConfig()
	}

	// Validate configuration
	if err := hfConfig.Validate(); err != nil {
		return fmt.Errorf("invalid Hellfire configuration: %w", err)
	}

	// Use port from config if not specified
	if port == 8888 {
		port = hfConfig.API.Port
	}

	// Initialize handlers
	_ = handlers.NewNetworkHandler()
	_ = handlers.NewFirewallHandler()
	_ = handlers.NewDHCPHandler(manager)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Initialize rate limiters
	globalLimiter := middleware.NewIPRateLimiter(
		hfConfig.RateLimit.GlobalRequestsPerMinute,
		hfConfig.RateLimit.GlobalBurst,
	)

	authLimiter := middleware.NewIPRateLimiter(
		hfConfig.RateLimit.AuthRequestsPerMinute,
		hfConfig.RateLimit.AuthBurst,
	)

	// Initialize CSRF manager
	csrfMgr := middleware.NewCSRFManager()

	// Start audit log cleanup scheduler (runs daily)
	if hfConfig.Audit.Enabled {
		// Run cleanup check once per day
		audit.StartCleanupScheduler(hfConfig.Audit.RetentionDays, 24*time.Hour)
	}

	// Start session cleanup scheduler (runs every hour)
	auth.StartSessionCleanupScheduler(1 * time.Hour)

	// Security headers middleware (should be early in the chain)
	r.Use(middleware.SecurityHeadersMiddleware())

	// CORS middleware (configured via Hellfire config)
	if hfConfig.API.EnableCORS {
		r.Use(corsMiddleware(hfConfig.API.AllowedOrigins))
	}

	// Request logging middleware (log all requests)
	r.Use(middleware.RequestLoggingMiddleware())

	// Content-Type validation
	r.Use(middleware.ContentTypeValidationMiddleware())

	// Global rate limiting
	r.Use(middleware.RateLimitMiddleware(globalLimiter))

	// Swagger documentation (protected with authentication if enabled)
	if hfConfig.Security.EnableSwagger {
		swaggerRoutes := r.Group("/api/docs")
		swaggerRoutes.Use(auth.AuthMiddleware()) // Require authentication
		{
			swaggerRoutes.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		}

		// OpenAPI JSON also requires auth
		r.GET("/api/openapi.json", auth.AuthMiddleware(), func(c *gin.Context) {
			c.File("./docs/swagger.json")
		})
	}

	// Health check (public)
	r.GET("/health", healthHandler)

	// Public API routes
	api := r.Group("/api")
	{
		// Bootstrap endpoint (public)
		api.GET("/bootstrap", bootstrapHandler)

		// Onboarding endpoint (public, only when no users exist)
		api.POST("/onboarding", middleware.RateLimitMiddleware(authLimiter), onboardingHandler)

		// Authentication endpoints
		api.GET("/auth/csrf", middleware.GetCSRFTokenHandler(csrfMgr)) // Get CSRF token
		api.POST("/auth/login", middleware.RateLimitMiddleware(authLimiter), loginHandler)
		api.POST("/auth/logout", auth.AuthMiddleware(), middleware.CSRFMiddleware(csrfMgr), logoutHandler)
		api.GET("/auth/me", auth.AuthMiddleware(), meHandler)

		// Protected config routes (requires authentication + CSRF for state changes)
		configRoutes := api.Group("/config", auth.AuthMiddleware())
		{
			// Read operations (no CSRF required)
			configRoutes.GET("/:name", getConfigHandler(manager))
			configRoutes.GET("/:name/:section", getSectionHandler(manager))
			configRoutes.GET("/:name/:section/:option", getOptionHandler(manager))
			configRoutes.GET("/changes", changesHandler(manager))

			// Write operations (CSRF required)
			configRoutes.PUT("/:name/:section/:option",
				middleware.CSRFMiddleware(csrfMgr),
				auth.RequireRole(db.RoleAdmin, db.RoleOperator),
				setOptionHandler(manager))

			configRoutes.POST("/commit",
				middleware.CSRFMiddleware(csrfMgr),
				auth.RequireRole(db.RoleAdmin, db.RoleOperator),
				commitHandler(manager))

			configRoutes.POST("/revert",
				middleware.CSRFMiddleware(csrfMgr),
				auth.RequireRole(db.RoleAdmin, db.RoleOperator),
				revertHandler(manager))

			configRoutes.POST("/validate",
				middleware.CSRFMiddleware(csrfMgr),
				auth.RequireRole(db.RoleAdmin, db.RoleOperator),
				validateHandler(manager))
		}
	}

	// Serve static files from web UI build (for production)
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/vite.svg", "./web/dist/vite.svg")

	// SPA fallback - serve index.html for all other routes
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting API server on %s\n", addr)
	return r.Run(addr)
}

// healthHandler godoc
// @Summary Health check
// @Description Check if the API server is running
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// getConfigHandler godoc
// @Summary Get configuration
// @Description Get entire configuration file
// @Tags config
// @Produce json
// @Param name path string true "Configuration name (e.g., network, firewall)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /config/{name} [get]
func getConfigHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		cfg, err := manager.Load(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, configToJSON(cfg))
	}
}

// getSectionHandler godoc
// @Summary Get configuration section
// @Description Get a specific section from configuration
// @Tags config
// @Produce json
// @Param name path string true "Configuration name"
// @Param section path string true "Section name or type"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /config/{name}/{section} [get]
func getSectionHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		section := c.Param("section")

		cfg, err := manager.Load(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Find section
		var sec *uci.Section
		for _, s := range cfg.Sections {
			if s.Name == section || s.Type == section {
				sec = s
				break
			}
		}

		if sec == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "section not found"})
			return
		}

		c.JSON(http.StatusOK, sectionToJSON(sec))
	}
}

// getOptionHandler godoc
// @Summary Get configuration option
// @Description Get a specific option value from a section
// @Tags config
// @Produce json
// @Param name path string true "Configuration name"
// @Param section path string true "Section name"
// @Param option path string true "Option key"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /config/{name}/{section}/{option} [get]
func getOptionHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		section := c.Param("section")
		option := c.Param("option")

		path := fmt.Sprintf("%s.%s.%s", name, section, option)
		value, err := manager.Get(path)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"value": value})
	}
}

// SetOptionRequest represents the request body for setting an option
type SetOptionRequest struct {
	Value string `json:"value" binding:"required" example:"192.168.1.1"`
}

// setOptionHandler godoc
// @Summary Set configuration option
// @Description Set a configuration option value (staged, requires commit)
// @Tags config
// @Accept json
// @Produce json
// @Param name path string true "Configuration name"
// @Param section path string true "Section name"
// @Param option path string true "Option key"
// @Param request body SetOptionRequest true "Option value"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /config/{name}/{section}/{option} [put]
func setOptionHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		section := c.Param("section")
		option := c.Param("option")

		var req SetOptionRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			apierrors.BadRequest(c, err)
			return
		}

		path := fmt.Sprintf("%s.%s.%s", name, section, option)
		if err := manager.Set(path, req.Value); err != nil {
			// Audit log failure
			user := auth.GetUser(c)
			username := "unknown"
			var userID *uint
			if user != nil {
				username = user.Username
				userID = &user.ID
			}
			audit.LogFailure(audit.ActionConfigWrite, userID, username, path,
				fmt.Sprintf("Failed to set %s", path), err)

			apierrors.OperationFailed(c, err)
			return
		}

		// Audit log success
		user := auth.GetUser(c)
		username := "unknown"
		var userID *uint
		if user != nil {
			username = user.Username
			userID = &user.ID
		}
		audit.LogSuccess(audit.ActionConfigWrite, userID, username, path,
			fmt.Sprintf("Set %s = %s (staged)", path, req.Value))

		// Publish event
		bus.Publish(bus.Event{
			Type:       bus.EventConfigChanged,
			ConfigName: name,
			Data:       map[string]string{"path": path, "value": req.Value},
		})

		c.JSON(http.StatusOK, gin.H{
			"message": "value staged, commit to apply",
			"path":    path,
			"value":   req.Value,
		})
	}
}

// commitHandler godoc
// @Summary Commit changes
// @Description Commit staged configuration changes to the system
// @Tags config
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /commit [post]
func commitHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.GetUser(c)
		username := "unknown"
		var userID *uint
		if user != nil {
			username = user.Username
			userID = &user.ID
		}

		if !manager.HasChanges() {
			c.JSON(http.StatusOK, gin.H{"message": "no changes to commit"})
			return
		}

		changes := manager.GetChanges()

		if err := manager.Commit(); err != nil {
			// Audit log failure
			audit.LogFailure(audit.ActionConfigCommit, userID, username, "config",
				"Failed to commit configuration changes", err)

			apierrors.OperationFailed(c, err)
			return
		}

		// Audit log success
		audit.LogSuccess(audit.ActionConfigCommit, userID, username, "config",
			fmt.Sprintf("Committed configuration changes: %v", changes))

		// Publish event
		bus.Publish(bus.Event{
			Type: bus.EventConfigCommitted,
			Data: changes,
		})

		c.JSON(http.StatusOK, gin.H{
			"message": "changes committed",
			"configs": changes,
		})
	}
}

// revertHandler godoc
// @Summary Revert changes
// @Description Revert all staged configuration changes
// @Tags config
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /revert [post]
func revertHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.GetUser(c)
		username := "unknown"
		var userID *uint
		if user != nil {
			username = user.Username
			userID = &user.ID
		}

		if !manager.HasChanges() {
			c.JSON(http.StatusOK, gin.H{"message": "no changes to revert"})
			return
		}

		changes := manager.GetChanges()

		if err := manager.Revert(); err != nil {
			// Audit log failure
			audit.LogFailure(audit.ActionConfigRevert, userID, username, "config",
				"Failed to revert configuration changes", err)

			apierrors.OperationFailed(c, err)
			return
		}

		// Audit log success
		audit.LogSuccess(audit.ActionConfigRevert, userID, username, "config",
			fmt.Sprintf("Reverted configuration changes: %v", changes))

		// Publish event
		bus.Publish(bus.Event{
			Type: bus.EventConfigReverted,
			Data: changes,
		})

		c.JSON(http.StatusOK, gin.H{
			"message": "changes reverted",
			"configs": changes,
		})
	}
}

// validateHandler godoc
// @Summary Validate staged changes
// @Description Validate staged configuration changes without applying them (dry-run)
// @Tags config
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /validate [post]
func validateHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.GetUser(c)
		username := "unknown"
		var userID *uint
		if user != nil {
			username = user.Username
			userID = &user.ID
		}

		if !manager.HasChanges() {
			c.JSON(http.StatusOK, gin.H{
				"valid":   true,
				"message": "no changes to validate",
			})
			return
		}

		changes := manager.GetChanges()

		// Perform validation by checking if the configuration would be valid
		// This is a dry-run check that validates syntax and basic constraints
		validationErrors := make(map[string][]string)
		allValid := true

		for _, configName := range changes {
			cfg, err := manager.Load(configName)
			if err != nil {
				validationErrors[configName] = append(validationErrors[configName], err.Error())
				allValid = false
				continue
			}

			// Additional validation can be added here
			// For now, if it loads without error, it's considered valid
			_ = cfg
		}

		// Audit log validation attempt
		if allValid {
			audit.LogSuccess(audit.ActionConfigRead, userID, username, "config",
				fmt.Sprintf("Validated configuration changes: %v", changes))

			c.JSON(http.StatusOK, gin.H{
				"valid":   true,
				"message": "all changes are valid",
				"configs": changes,
			})
		} else {
			audit.LogFailure(audit.ActionConfigRead, userID, username, "config",
				"Configuration validation failed", fmt.Errorf("validation errors: %v", validationErrors))

			c.JSON(http.StatusBadRequest, gin.H{
				"valid":  false,
				"errors": validationErrors,
			})
		}
	}
}

// changesHandler godoc
// @Summary Get staged changes
// @Description Get list of staged configuration changes
// @Tags config
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /changes [get]
func changesHandler(manager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		changes := manager.GetChanges()

		c.JSON(http.StatusOK, gin.H{
			"has_changes": manager.HasChanges(),
			"configs":     changes,
		})
	}
}

// configToJSON converts UCI config to JSON-friendly map
func configToJSON(cfg *uci.Config) map[string]interface{} {
	result := make(map[string]interface{})
	typeCounts := make(map[string]int)

	for _, section := range cfg.Sections {
		var key string

		if section.Name != "" {
			// Named section: use the name as key
			key = section.Name
		} else {
			// Unnamed section: use type with index (e.g., "rule_0", "rule_1", "zone_0")
			count := typeCounts[section.Type]
			key = fmt.Sprintf("%s_%d", section.Type, count)
			typeCounts[section.Type]++
		}

		result[key] = sectionToJSON(section)
	}

	return result
}

// sectionToJSON converts UCI section to JSON-friendly map
func sectionToJSON(section *uci.Section) map[string]interface{} {
	result := make(map[string]interface{})

	// Add section type (important for parsing)
	result[".type"] = section.Type
	if section.Name != "" {
		result[".name"] = section.Name
	}

	// Add options
	for k, v := range section.Options {
		result[k] = v
	}

	// Add lists
	for k, v := range section.Lists {
		result[k] = v
	}

	return result
}

// Authentication Handlers

// csrfTokenHandler godoc
// @Summary Get CSRF token
// @Description Get a CSRF token for making authenticated state-changing requests
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]string
// @Router /auth/csrf [get]
func csrfTokenHandler(c *gin.Context) {
	// This is just a placeholder - actual implementation is in middleware.GetCSRFTokenHandler
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	User      *db.User  `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// loginHandler godoc
// @Summary User login
// @Description Authenticate user and get session token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body loginRequest true "Login credentials"
// @Success 200 {object} loginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func loginHandler(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.BadRequest(c, err)
		return
	}

	// Get client IP
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Attempt login
	session, err := auth.Login(req.Username, req.Password, ipAddress, userAgent)
	if err != nil {
		// Audit log failed login attempt
		audit.LogFailure(audit.ActionUserLogin, nil, req.Username, "auth",
			fmt.Sprintf("Failed login attempt from %s", ipAddress), err)

		apierrors.Unauthorized(c, err)
		return
	}

	// Audit log successful login
	audit.LogSuccess(audit.ActionUserLogin, &session.UserID, req.Username, "auth",
		fmt.Sprintf("User logged in from %s", ipAddress))

	c.JSON(http.StatusOK, loginResponse{
		Token:     session.Token,
		User:      &session.User,
		ExpiresAt: session.ExpiresAt,
	})
}

// logoutHandler godoc
// @Summary User logout
// @Description Invalidate current session
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/logout [post]
// @Security BearerAuth
func logoutHandler(c *gin.Context) {
	session := auth.GetSession(c)
	if session == nil {
		apierrors.Unauthorized(c, fmt.Errorf("no session"))
		return
	}

	user := auth.GetUser(c)
	username := "unknown"
	var userID *uint
	if user != nil {
		username = user.Username
		userID = &user.ID
	}

	// Delete session
	if err := auth.DeleteSession(session.Token); err != nil {
		// Audit log failure
		audit.LogFailure(audit.ActionUserLogout, userID, username, "auth",
			"Failed to logout", err)

		apierrors.OperationFailed(c, err)
		return
	}

	// Audit log successful logout
	ipAddress := c.ClientIP()
	audit.LogSuccess(audit.ActionUserLogout, userID, username, "auth",
		fmt.Sprintf("User logged out from %s", ipAddress))

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// meHandler godoc
// @Summary Get current user
// @Description Get information about the current authenticated user
// @Tags auth
// @Produce json
// @Success 200 {object} db.User
// @Failure 401 {object} map[string]string
// @Router /auth/me [get]
// @Security BearerAuth
func meHandler(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	// Include permissions
	permissions := auth.GetUserPermissions(user)

	c.JSON(http.StatusOK, gin.H{
		"user":        user,
		"permissions": permissions,
	})
}

// bootstrapHandler godoc
// @Summary System bootstrap metadata
// @Description Get system metadata including initialization status
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /bootstrap [get]
func bootstrapHandler(c *gin.Context) {
	userCount, err := db.CountUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check system status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"initialized": userCount > 0,
		"version":     "1.0.0",
		"system":      "hellfire",
	})
}

type onboardingRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// onboardingHandler godoc
// @Summary Create initial admin user
// @Description Create the first admin user during system onboarding
// @Tags system
// @Accept json
// @Produce json
// @Param request body onboardingRequest true "Admin user details"
// @Success 200 {object} loginResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /onboarding [post]
func onboardingHandler(c *gin.Context) {
	// Check if system is already initialized
	userCount, err := db.CountUsers()
	if err != nil {
		apierrors.OperationFailed(c, err)
		return
	}

	if userCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "system already initialized"})
		return
	}

	var req onboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.BadRequest(c, err)
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		apierrors.OperationFailed(c, err)
		return
	}

	// Create admin user
	user := &db.User{
		Username:     req.Email,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Role:         db.RoleAdmin,
		Enabled:      true,
	}

	if err := db.CreateUser(user); err != nil {
		apierrors.OperationFailed(c, err)
		return
	}

	// Create session for immediate login
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	session, err := auth.CreateSession(user.ID, ipAddress, userAgent, 0)
	if err != nil {
		apierrors.OperationFailed(c, err)
		return
	}

	// Load user into session
	session.User = *user

	// Audit log
	audit.LogSuccess(audit.ActionUserCreate, &user.ID, user.Username, "onboarding",
		fmt.Sprintf("Initial admin user created from %s", ipAddress))

	c.JSON(http.StatusOK, loginResponse{
		Token:     session.Token,
		User:      &session.User,
		ExpiresAt: session.ExpiresAt,
	})
}

// corsMiddleware creates a CORS middleware with specified allowed origins
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-CSRF-Token")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
