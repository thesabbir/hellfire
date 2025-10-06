import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import Cookies from 'js-cookie'
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { postOnboarding } from "@/lib/api";
import { AUTH_COOKIE_NAME } from "@/lib/api-client";
import { Flame, Rocket } from "lucide-react";

export function OnboardingForm({
  className,
  ...props
}: React.ComponentProps<"div">) {
  const navigate = useNavigate();
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");

    const formData = new FormData(e.currentTarget);
    const name = formData.get("name") as string;
    const email = formData.get("email") as string;
    const password = formData.get("password") as string;
    const confirmPassword = formData.get("confirm-password") as string;

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    setLoading(true);

    try {
      const response = await postOnboarding({
        body: { name, email, password },
      });

      if (response.error) {
        const errorObj = response.error as { error?: string }
        setError(errorObj.error || "Failed to create admin account");
        return;
      }

      // Store token in cookie
      if (response.data?.token) {
        Cookies.set(AUTH_COOKIE_NAME, response.data.token, {
          expires: 7, // 7 days
          sameSite: 'strict',
          secure: window.location.protocol === 'https:',
        })
      }

      // Redirect to dashboard
      navigate({ to: "/" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card className="w-full">
        <CardHeader className="space-y-1 flex flex-col items-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-primary text-primary-foreground mb-2">
            <Rocket className="h-7 w-7" />
          </div>
          <CardTitle className="text-2xl font-bold text-center">Welcome to Hellfire</CardTitle>
          <CardDescription className="text-center">
            Create your administrator account to get started
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <div className="bg-destructive/10 border border-destructive/20 text-destructive rounded-lg p-3 text-sm">
                {error}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="admin-name">Full Name</Label>
              <Input
                id="admin-name"
                name="name"
                type="text"
                placeholder="John Doe"
                required
                disabled={loading}
                autoComplete="name"
                className="h-11"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="admin-email">Email Address</Label>
              <Input
                id="admin-email"
                name="email"
                type="email"
                placeholder="admin@example.com"
                required
                disabled={loading}
                autoComplete="email"
                className="h-11"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="admin-password">Password</Label>
              <Input
                id="admin-password"
                name="password"
                type="password"
                placeholder="At least 8 characters"
                required
                minLength={8}
                disabled={loading}
                autoComplete="new-password"
                className="h-11"
              />
              <p className="text-xs text-muted-foreground">
                Use a strong password with at least 8 characters
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="admin-confirm-password">Confirm Password</Label>
              <Input
                id="admin-confirm-password"
                name="confirm-password"
                type="password"
                placeholder="Re-enter your password"
                required
                minLength={8}
                disabled={loading}
                autoComplete="new-password"
                className="h-11"
              />
            </div>

            <Button
              type="submit"
              className="w-full h-11 mt-6"
              disabled={loading}
              size="lg"
            >
              {loading ? (
                <>
                  <div className="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-background border-t-transparent" />
                  Creating Account...
                </>
              ) : (
                <>
                  <Flame className="mr-2 h-4 w-4" />
                  Create Administrator Account
                </>
              )}
            </Button>

            <p className="text-xs text-center text-muted-foreground mt-4">
              This account will have full administrative access to the system
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
