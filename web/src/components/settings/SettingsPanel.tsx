"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useState, useEffect } from "react";
import { Settings as SettingsIcon, Save } from "lucide-react";
import { api } from "@/lib/api";

interface SettingsPanelProps {
  refreshInterval: number;
  setRefreshInterval: (value: number) => void;
  autoRefresh: boolean;
  setAutoRefresh: (value: boolean) => void;
}

export function SettingsPanel({ 
  refreshInterval, 
  setRefreshInterval, 
  autoRefresh, 
  setAutoRefresh 
}: SettingsPanelProps) {
  const [seedUrl, setSeedUrl] = useState("http://localhost:8484");

  useEffect(() => {
    const savedSettings = localStorage.getItem("limedb_settings");
    if (savedSettings) {
      const settings = JSON.parse(savedSettings);
      if (settings.seedUrl) {
        setSeedUrl(settings.seedUrl);
      }
    }
  }, []);

  const handleSave = () => {
    // Save settings to localStorage
    localStorage.setItem("limedb_settings", JSON.stringify({
      refreshInterval,
      autoRefresh,
      seedUrl,
    }));
    
    // Update API client
    api.setSeedUrl(seedUrl);
    
    // Reload to ensure fresh state
    window.location.reload();
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
        <p className="text-muted-foreground">
          Configure your dashboard preferences
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <SettingsIcon className="h-5 w-5" />
              Data Refresh
            </CardTitle>
            <CardDescription>
              Configure how often the dashboard fetches new data
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="refresh-interval">Refresh Interval</Label>
              <Select 
                value={refreshInterval.toString()} 
                onValueChange={(value) => setRefreshInterval(parseInt(value))}
              >
                <SelectTrigger id="refresh-interval">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1000">1 second</SelectItem>
                  <SelectItem value="2000">2 seconds</SelectItem>
                  <SelectItem value="5000">5 seconds</SelectItem>
                  <SelectItem value="10000">10 seconds</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auto-refresh">Auto Refresh</Label>
              <Switch
                id="auto-refresh"
                checked={autoRefresh}
                onCheckedChange={setAutoRefresh}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cluster Connection</CardTitle>
            <CardDescription>
              Configure the seed node URL for cluster discovery
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="seed-url">Seed Node URL</Label>
              <Input
                id="seed-url"
                value={seedUrl}
                onChange={(e) => setSeedUrl(e.target.value)}
                placeholder="http://localhost:8484"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <Button onClick={handleSave} className="w-full md:w-auto">
        <Save className="mr-2 h-4 w-4" />
        Save Settings
      </Button>
    </div>
  );
}
