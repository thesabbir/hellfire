import { createFileRoute } from '@tanstack/react-router'
import { OnboardingForm } from '@/components/onboarding-form'

export const Route = createFileRoute('/onboarding')({
  component: OnboardingPage,
})

function OnboardingPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background via-background to-muted p-4">
      <div className="w-full max-w-md">
        <OnboardingForm />
      </div>
    </div>
  )
}
