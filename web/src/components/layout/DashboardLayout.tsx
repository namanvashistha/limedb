"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { 
  LayoutDashboard, 
  Database, 
  Activity, 
  Settings, 
  Menu, 
  Server,
  Network
} from "lucide-react";
import { ThemeToggle } from "@/components/theme-toggle";
import { ConnectionDialog } from "@/components/dashboard/ConnectionDialog";
import { ClusterStatusBar } from "@/components/layout/ClusterStatusBar";

interface DashboardLayoutProps {
  children: React.ReactNode;
  activeTab: string;
  setActiveTab: (tab: string) => void;
  headerAction?: React.ReactNode;
}

export function DashboardLayout({ 
  children, 
  activeTab, 
  setActiveTab,
  headerAction 
}: DashboardLayoutProps) {
  const [open, setOpen] = useState(false);

  const sidebarItems = [
    {
      title: "Cluster",
      items: [
        { id: "overview", label: "Overview", icon: LayoutDashboard },
        { id: "nodes", label: "Nodes", icon: Server },
        { id: "metrics", label: "Topology", icon: Network },
      ]
    },
    {
      title: "Data",
      items: [
        { id: "explorer", label: "Explorer", icon: Database },
      ]
    },
    {
      title: "System",
      items: [
        { id: "settings", label: "Settings", icon: Settings },
      ]
    }
  ];

  const SidebarContent = () => (
    <div className="flex flex-col h-full">
      <div className="h-14 flex items-center px-6 border-b">
        <div className="flex items-center gap-2 font-semibold">
          <div className="h-6 w-6 rounded-full bg-lime-500 flex items-center justify-center">
            <Server className="h-3 w-3 text-black fill-current" />
          </div>
          <span>LimeDB</span>
        </div>
      </div>
      <ScrollArea className="flex-1 py-4">
        <div className="px-4 space-y-6">
          {sidebarItems.map((group, i) => (
            <div key={i} className="space-y-2">
              <h3 className="px-2 text-xs font-semibold text-muted-foreground tracking-wider uppercase">
                {group.title}
              </h3>
              <div className="space-y-1">
                {group.items.map((item) => (
                  <Button
                    key={item.id}
                    variant={activeTab === item.id ? "secondary" : "ghost"}
                    className={cn(
                      "w-full justify-start gap-2",
                      activeTab === item.id && "bg-secondary"
                    )}
                    onClick={() => {
                      setActiveTab(item.id);
                      setOpen(false);
                    }}
                  >
                    <item.icon className="h-4 w-4" />
                    {item.label}
                  </Button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
      <ClusterStatusBar />
    </div>
  );

  return (
    <div className="flex min-h-screen bg-background">
      {/* Desktop Sidebar */}
      <div className="hidden md:flex w-64 flex-col border-r bg-card">
        <SidebarContent />
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-h-screen">
        <header className="h-14 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 flex items-center justify-between px-6 sticky top-0 z-50">
          <div className="flex items-center gap-4">
            <Sheet open={open} onOpenChange={setOpen}>
              <SheetTrigger asChild className="md:hidden">
                <Button variant="ghost" size="icon">
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent side="left" className="p-0 w-64">
                <SidebarContent />
              </SheetContent>
            </Sheet>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <span className="hidden md:inline">Dashboard</span>
              <span className="hidden md:inline">/</span>
              <span className="font-medium text-foreground capitalize">
                {sidebarItems.flatMap(g => g.items).find(i => i.id === activeTab)?.label || activeTab}
              </span>
            </div>
          </div>
          
          <div className="flex items-center gap-3">
            <ThemeToggle />
            {headerAction}
          </div>
        </header>

        <main className="flex-1 px-6 pt-6 overflow-hidden flex flex-col">
          <div className="flex-1 flex flex-col">
            {children}
          </div>
        </main>
      </div>
      <ConnectionDialog />
    </div>
  );
}
