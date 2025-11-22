import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RingState } from "@/lib/types";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

interface RingVisualizerProps {
  ring: RingState;
}

export function RingVisualizer({ ring }: RingVisualizerProps) {
  const ranges = ring.ranges || {};
  const nodes = Object.keys(ranges);
  
  let totalTokens = 0;
  const parsedRanges: { node: string; size: number; color: string }[] = [];
  
  // Colors for nodes
  const colors = ["bg-lime-500", "bg-blue-500", "bg-purple-500", "bg-cyan-500", "bg-yellow-500", "bg-red-500"];

  nodes.forEach((node, index) => {
    let nodeSize = 0;
    ranges[node].forEach((r) => {
      nodeSize += r.size;
    });
    totalTokens += nodeSize;
    if (nodeSize > 0) {
      parsedRanges.push({
        node,
        size: nodeSize,
        color: colors[index % colors.length],
      });
    }
  });

  parsedRanges.sort((a, b) => a.node.localeCompare(b.node));

  return (
    <Card>
      <CardHeader>
        <CardTitle>Ring Distribution</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Visual Bar */}
          <div className="flex h-8 w-full overflow-hidden rounded-full bg-secondary">
            <TooltipProvider>
              {parsedRanges.map((range) => {
                const share = (range.size / totalTokens) * 100;
                return (
                  <Tooltip key={range.node}>
                    <TooltipTrigger asChild>
                      <div
                        className={`h-full ${range.color} transition-all hover:opacity-80`}
                        style={{ width: `${share}%` }}
                      />
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="font-bold">{range.node}</p>
                      <p>Share: {share.toFixed(1)}%</p>
                      <p>Tokens: {range.size}</p>
                    </TooltipContent>
                  </Tooltip>
                );
              })}
            </TooltipProvider>
          </div>

          {/* Legend */}
          <div className="grid grid-cols-2 gap-2 text-sm sm:grid-cols-3">
            {parsedRanges.map((range) => (
              <div key={range.node} className="flex items-center gap-2">
                <div className={`h-3 w-3 rounded-full ${range.color}`} />
                <span className="truncate">{range.node}</span>
                <span className="text-muted-foreground">
                  {((range.size / totalTokens) * 100).toFixed(1)}%
                </span>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
