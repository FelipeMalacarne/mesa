import { Moon, Sun } from "lucide-react";

import { Button } from "@/ui/Button";

import { useTheme } from "./theme";

export function ThemeToggle({ size = "md" }: { size?: "sm" | "md" | "xs" }) {
  const theme = useTheme((s) => s.theme);
  const toggle = useTheme((s) => s.toggle);
  return (
    <Button variant="ghost" size={size} icon onClick={toggle} aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} title="Toggle theme">
      {theme === "dark" ? <Sun className="m-icon m-icon--sm" aria-hidden="true" /> : <Moon className="m-icon m-icon--sm" aria-hidden="true" />}
    </Button>
  );
}
