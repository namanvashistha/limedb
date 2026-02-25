"use client";

import { useEffect, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export function ConnectionDialog() {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState("");

  useEffect(() => {
    // Check if seed URL is set in localStorage
    const savedSettings = localStorage.getItem("limedb_settings");
    if (savedSettings) {
      const settings = JSON.parse(savedSettings);
      if (settings.seedUrl) {
        api.setSeedUrl(settings.seedUrl);
        return;
      }
    }
    
    // If not set, open dialog
    setOpen(true);
  }, []);

  const handleConnect = () => {
    if (!url) return;
    
    // Save to localStorage
    const savedSettings = localStorage.getItem("limedb_settings");
    const settings = savedSettings ? JSON.parse(savedSettings) : {};
    
    const newSettings = {
      ...settings,
      seedUrl: url
    };
    
    localStorage.setItem("limedb_settings", JSON.stringify(newSettings));
    
    // Update API client
    api.setSeedUrl(url);
    
    setOpen(false);
    
    // Refresh page to ensure everything uses new URL
    window.location.reload();
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-[425px]" onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Connect to Cluster</DialogTitle>
          <DialogDescription>
            Enter the URL of a LimeDB node to connect to the cluster.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="url" className="text-right">
              Node URL
            </Label>
            <Input
              id="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="http://node1:8484"
              className="col-span-3"
            />
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleConnect} disabled={!url}>Connect</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
