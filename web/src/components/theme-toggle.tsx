import { Monitor, Moon, Sun } from "lucide-react";

import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useTheme } from "@/providers/theme-provider";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();

  return (
    <ToggleGroup
      type="single"
      value={theme}
      onValueChange={(value) => value && setTheme(value as typeof theme)}
      variant="outline"
      size="sm"
    >
      <ToggleGroupItem value="light" aria-label="Switch to light theme">
        <Sun className="size-4" />
      </ToggleGroupItem>
      <ToggleGroupItem value="system" aria-label="Use system theme">
        <Monitor className="size-4" />
      </ToggleGroupItem>
      <ToggleGroupItem value="dark" aria-label="Switch to dark theme">
        <Moon className="size-4" />
      </ToggleGroupItem>
    </ToggleGroup>
  );
}
