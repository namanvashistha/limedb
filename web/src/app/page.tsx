"use client";

import { useState } from "react";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { ClusterOverview } from "@/components/dashboard/ClusterOverview";
import { NodesList } from "@/components/dashboard/NodesList";
import { NodeDetailView } from "@/components/dashboard/NodeDetailView";
import { EnhancedDataExplorer } from "@/components/dashboard/EnhancedDataExplorer";
import { NetworkTopology } from "@/components/topology/NetworkTopology";
import { SettingsPanel } from "@/components/settings/SettingsPanel";
import { CommandPalette } from "@/components/command-palette";
import { Button } from "@/components/ui/button";
import { RefreshCw, Server } from "lucide-react";

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState("overview");
  const [selectedNode, setSelectedNode] = useState<string | null>(null);

  // Handle navigation
  const handleNavigate = (tab: string) => {
    setActiveTab(tab);
    setSelectedNode(null); // Reset selected node when changing tabs
  };

  const handleSelectNode = (nodeUrl: string) => {
    setSelectedNode(nodeUrl);
    // Ideally we would switch to a "nodes" tab or similar, but here we just render the detail view
    // if we are in the nodes tab context.
    // If we are in overview, we might want to switch to nodes tab first.
    if (activeTab !== "nodes") {
      setActiveTab("nodes");
    }
  };

  const handleBackToNodes = () => {
    setSelectedNode(null);
  };

  const renderContent = () => {
    switch (activeTab) {
      case "overview":
        return <ClusterOverview />;
      case "nodes":
        return (
          <div className="flex gap-6 h-[calc(100vh-12rem)]">
            {/* Left Pane - Nodes List */}
            <div className="w-1/3 min-w-[300px] overflow-hidden">
              <NodesList 
                onSelectNode={handleSelectNode} 
                selectedNodeUrl={selectedNode}
                compact={true}
              />
            </div>
            {/* Right Pane - Node Details */}
            <div className="flex-1 overflow-auto">
              {selectedNode ? (
                <NodeDetailView 
                  nodeUrl={selectedNode} 
                  showBackButton={false}
                />
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
                  <Server className="h-16 w-16 mb-4 opacity-20" />
                  <p className="text-lg font-medium">Select a node to view details</p>
                  <p className="text-sm">Click on any node from the list to see its metrics and status</p>
                </div>
              )}
            </div>
          </div>
        );
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
