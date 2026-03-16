"use client";

import { useState } from "react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { ClusterOverview } from "@/components/dashboard/ClusterOverview";
import { NodesFleetView } from "@/components/dashboard/NodesFleetView";
import { EnhancedDataExplorer } from "@/components/dashboard/EnhancedDataExplorer";
import { NetworkTopology } from "@/components/topology/NetworkTopology";
import { SettingsPanel } from "@/components/settings/SettingsPanel";
import { CommandPalette } from "@/components/command-palette";
import { Button } from "@/components/ui/button";
import { RefreshCw } from "lucide-react";

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState("overview");

  const handleNavigate = (tab: string) => setActiveTab(tab);

  const renderContent = () => {
    switch (activeTab) {
      case "overview":
        return <ClusterOverview />;
      case "nodes":
        return <NodesFleetView />;
      case "metrics":
        return (
          <div className="h-[calc(100vh-12rem)]">
            <NetworkTopology />
          </div>
        );
      case "explorer":
        return <EnhancedDataExplorer />;
      case "settings":
        return <SettingsPanel />;
      default:
        return <ClusterOverview />;
    }
  };

  return (
    <DashboardLayout 
      activeTab={activeTab} 
      setActiveTab={handleNavigate}
      headerAction={
        <div className="flex items-center gap-2">
           <Button variant="outline" size="sm" onClick={() => window.location.reload()}>
             <RefreshCw className="h-4 w-4 mr-2" />
             Refresh
           </Button>
           <CommandPalette onNavigate={handleNavigate} onRefresh={() => window.location.reload()} />
        </div>
      }
    >
      {renderContent()}
    </DashboardLayout>
  );
}
