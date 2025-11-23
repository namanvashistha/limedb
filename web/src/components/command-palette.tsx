"use client";

import * as React from "react";
import { Calculator, Calendar, CreditCard, Settings, Smile, User, RefreshCw, Moon, Sun, Database, Activity, Clock } from "lucide-react";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { useTheme } from "next-themes";

interface CommandPaletteProps {
  onNavigate: (tab: string) => void;
  onRefresh: () => void;
}

export function CommandPalette({ onNavigate, onRefresh }: CommandPaletteProps) {
  const [open, setOpen] = React.useState(false);
  const { setTheme, theme } = useTheme();

  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((open) => !open);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  const runCommand = React.useCallback((command: () => void) => {
    setOpen(false);
    command();
  }, []);

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="Type a command or search..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        
        <CommandGroup heading="Navigation">
          <CommandItem onSelect={() => runCommand(() => onNavigate("overview"))}>
            <Activity className="mr-2 h-4 w-4" />
            <span>Overview</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => onNavigate("metrics"))}>
            <Activity className="mr-2 h-4 w-4" />
            <span>Metrics</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => onNavigate("explorer"))}>
            <Database className="mr-2 h-4 w-4" />
            <span>Data Explorer</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => onNavigate("events"))}>
            <Clock className="mr-2 h-4 w-4" />
            <span>Events</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => onNavigate("settings"))}>
            <Settings className="mr-2 h-4 w-4" />
            <span>Settings</span>
          </CommandItem>
        </CommandGroup>
        
        <CommandSeparator />
        
        <CommandGroup heading="Actions">
          <CommandItem onSelect={() => runCommand(onRefresh)}>
            <RefreshCw className="mr-2 h-4 w-4" />
            <span>Refresh Data</span>
            <span className="ml-auto text-xs text-muted-foreground">⌘R</span>
          </CommandItem>
        </CommandGroup>
        
        <CommandSeparator />
        
        <CommandGroup heading="Theme">
          <CommandItem onSelect={() => runCommand(() => setTheme("light"))}>
            <Sun className="mr-2 h-4 w-4" />
            <span>Light Mode</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => setTheme("dark"))}>
            <Moon className="mr-2 h-4 w-4" />
            <span>Dark Mode</span>
          </CommandItem>
          <CommandItem onSelect={() => runCommand(() => setTheme("system"))}>
            <Settings className="mr-2 h-4 w-4" />
            <span>System</span>
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
