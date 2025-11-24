import * as React from "react"
import { Check, ChevronsUpDown, Server } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

interface NodeSelectorProps {
  nodes: string[]
  selectedNode: string
  onSelectNode: (node: string) => void
}

export function NodeSelector({ nodes, selectedNode, onSelectNode }: NodeSelectorProps) {
  const [open, setOpen] = React.useState(false)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-[250px] justify-between"
        >
          <div className="flex items-center gap-2 truncate">
            <Server className="h-4 w-4 shrink-0 opacity-50" />
            {selectedNode ? selectedNode : "Select node..."}
          </div>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[250px] p-0">
        <Command>
          <CommandInput placeholder="Search nodes..." />
          <CommandList>
            <CommandEmpty>No node found.</CommandEmpty>
            <CommandGroup heading="Available Nodes">
              {nodes.map((node) => (
                <CommandItem
                  key={node}
                  value={node}
                  onSelect={(currentValue) => {
                    onSelectNode(currentValue)
                    setOpen(false)
                  }}
                >
                  <Check
                    className={cn(
                      "mr-2 h-4 w-4",
                      selectedNode === node ? "opacity-100" : "opacity-0"
                    )}
                  />
                  {node}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
