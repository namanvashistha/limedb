from textual.app import App, ComposeResult
from textual.widgets import (
    Header,
    Footer,
    Static,
    DataTable,
    Label,
    TabbedContent,
    TabPane,
    Input,
    Button,
    Pretty,
)
from textual.containers import Container, Horizontal, Grid
from client import ClusterClient
from textual import log
import asyncio
import statistics
import sys
import argparse

# Theme Colors
LIME_GREEN = "#84cc16"
DARK_BG = "#121212"
MUTED_GREEN = "#3f6212"
WHITE = "#ffffff"
ERROR_RED = "#ef4444"

ASCII_LOGO = r"""
  _    _           ___  ___ 
 | |  (_)_ __  ___|   \| _ )
 | |__| | '  \/ -_) |) | _ \
 |____|_|_|_|_\___|___/|___/     
"""


class Logo(Static):
    """Displays the LimeDB ASCII Logo."""

    def render(self) -> str:
        return f"[{LIME_GREEN}]{ASCII_LOGO}[/]"


class CyclicDataTable(DataTable):
    """A DataTable that wraps selection from top-to-bottom and vice-versa."""

    def action_cursor_down(self) -> None:
        if self.cursor_row == self.row_count - 1:
            self.move_cursor(row=0)
        else:
            super().action_cursor_down()

    def action_cursor_up(self) -> None:
        if self.cursor_row == 0:
            self.move_cursor(row=self.row_count - 1)
        else:
            super().action_cursor_up()


class GossipDetails(Static):
    """A widget to display detailed gossip protocol information."""

    def compose(self) -> ComposeResult:
        yield Label("GOSSIP PROTOCOL DETAILS", classes="section-title")
        yield Container(
            Label("Select a node from the cluster table to view gossip details", id="gossip_info"),
            CyclicDataTable(id="gossip_peers_table"),
            classes="gossip-container"
        )

    def on_mount(self) -> None:
        table = self.query_one("#gossip_peers_table")
        table.add_columns("PEER URL", "HEARTBEAT", "LAG", "STATUS")
        table.cursor_type = "row"
        table.show_header = True

    def update_gossip_details(self, node_url: str, gossip_data: dict) -> None:
        info_label = self.query_one("#gossip_info")
        table = self.query_one("#gossip_peers_table")
        table.clear()

        if "error" in gossip_data or not gossip_data:
            info_label.update(f"[red]No gossip data available for {node_url}[/]")
            return

        # Check if standalone
        status = gossip_data.get("status", "")
        if status == "standalone":
            info_label.update(
                f"[bold white]{node_url}[/] - [blue]STANDALONE MODE[/]\n"
                f"[dim]Node running independently with no cluster peers[/]\n"
                f"[dim]Heartbeat:[/] {gossip_data.get('node_heartbeat', 0)}"
            )
            return

        # Display gossip summary
        cluster_health = gossip_data.get("cluster_health", "unknown")
        node_heartbeat = gossip_data.get("node_heartbeat", 0)
        total_peers = gossip_data.get("total_peers", 0)
        active_peers = gossip_data.get("active_peers", 0)
        dead_peers = gossip_data.get("dead_peers", 0)
        stale_peers = gossip_data.get("stale_peers", 0)
        convergence_rate = gossip_data.get("convergence_rate", 0)
        avg_lag = gossip_data.get("average_lag", 0)
        max_lag = gossip_data.get("max_lag", 0)

        # Health color coding
        if cluster_health == "healthy":
            health_colored = f"[green]✓ {cluster_health.upper()}[/]"
        elif cluster_health == "degraded":
            health_colored = f"[yellow]⚠ {cluster_health.upper()}[/]"
        elif cluster_health == "critical":
            health_colored = f"[red]✗ {cluster_health.upper()}[/]"
        else:
            health_colored = f"[dim]{cluster_health.upper()}[/]"

        info_text = (
            f"[bold white]{node_url}[/] - {health_colored}\n"
            f"[dim]Heartbeat:[/] {node_heartbeat} | "
            f"[dim]Peers:[/] {active_peers}/{total_peers} active | "
            f"[dim]Convergence:[/] {convergence_rate:.1f}%\n"
            f"[dim]Lag:[/] Avg {avg_lag:.1f} | Max {max_lag} | "
            f"[dim]Dead:[/] {dead_peers} | [dim]Stale:[/] {stale_peers}"
        )
        info_label.update(info_text)

        # Show peer details
        peer_details = gossip_data.get("peer_details", [])
        if not peer_details:
            table.add_row("[dim]No peer data available[/]", "-", "-", "-")
            return
            
        for peer in peer_details:
            url = peer.get("url", "unknown")
            heartbeat = peer.get("heartbeat", 0)
            lag = peer.get("lag", 0)
            status = peer.get("status", "unknown")

            # Status color coding
            if status == "active":
                status_colored = f"[green]● {status.upper()}[/]"
            elif status == "stale":
                status_colored = f"[yellow]● {status.upper()}[/]"
            elif status == "dead":
                status_colored = f"[red]● {status.upper()}[/]"
            else:
                status_colored = f"[dim]● {status.upper()}[/]"

            # Lag color coding
            if lag == 0:
                lag_colored = f"[green]{lag}[/]"
            elif lag <= 5:
                lag_colored = f"[yellow]{lag}[/]"
            else:
                lag_colored = f"[red]{lag}[/]"

            table.add_row(
                f"[bold white]{url}[/]",
                str(heartbeat),
                lag_colored,
                status_colored
            )


class ClusterStatus(Static):
    """A widget to display the status and metrics of all nodes."""

    def compose(self) -> ComposeResult:
        yield Label("CLUSTER NODES & METRICS", classes="section-title")
        yield CyclicDataTable(id="status_table")

    def on_mount(self) -> None:
        table = self.query_one(CyclicDataTable)
        table.add_columns(
            "STATUS", "NODE URL", "PORT", "PEERS", "GOSSIP", "CPU", "MEM", "UPTIME", "LATENCY"
        )
        table.cursor_type = "row"
        table.zebra_stripes = False
        table.show_header = True

    def update_status(self, status_data: dict, metrics_data: dict, gossip_data: dict) -> None:
        table = self.query_one(CyclicDataTable)
        # Save cursor position
        cursor_row = table.cursor_row
        table.clear()

        sorted_urls = sorted(status_data.keys())

        for url in sorted_urls:
            data = status_data[url]
            metrics = metrics_data.get(url, {})
            gossip = gossip_data.get(url, {})

            # Status Icon
            if "error" in data:
                status_icon = f"[{ERROR_RED}]● DOWN[/]"
                node_url = url
                peers_count = "-"
            else:
                node_url = str(data.get("nodeUrl", url))
                status = data.get("status", "UNKNOWN")
                peers_count = str(len(data.get("peers", [])))

                if status == "active":
                    status_icon = f"[{LIME_GREEN}]● ACTIVE[/]"
                else:
                    status_icon = f"[{ERROR_RED}]● {status.upper()}[/]"

            # Extract port from URL for display
            try:
                port = url.split(":")[-1]
            except:
                port = "?"

            # Gossip Status
            if 0 and "error" in gossip or not gossip:
                gossip_status = "-"
            else:
                cluster_health = gossip.get("cluster_health", "unknown")
                active_peers = gossip.get("active_peers", 0)
                total_peers = gossip.get("total_peers", 0)
                
                if cluster_health == "healthy":
                    gossip_status = f"[green]✓ {active_peers}/{total_peers}[/]"
                elif cluster_health == "degraded":
                    gossip_status = f"[yellow]⚠ {active_peers}/{total_peers}[/]"
                elif cluster_health == "critical":
                    gossip_status = f"[red]✗ {active_peers}/{total_peers}[/]"
                else:
                    gossip_status = f"[dim]? {active_peers}/{total_peers}[/]"

            # Metrics
            if "error" in metrics or not metrics:
                cpu = "-"
                mem = "-"
                uptime = "-"
                latency = "-"
            else:
                cpu = f"{metrics.get('cpu', 0) * 100:.1f}%"
                mem_mb = metrics.get("memory", 0) / (1024 * 1024)
                mem = f"{mem_mb:.0f} MB"
                uptime_s = metrics.get("uptime", 0)
                uptime = f"{int(uptime_s // 3600)}h {int((uptime_s % 3600) // 60)}m"

                lat_val = metrics.get("latency", 999)
                latency_str = f"{lat_val:.0f} ms"
                if lat_val < 10:
                    latency = f"[green]{latency_str}[/]"
                elif lat_val < 50:
                    latency = f"[yellow]{latency_str}[/]"
                else:
                    latency = f"[red]{latency_str}[/]"

            table.add_row(
                status_icon,
                f"[bold white]{node_url}[/]",
                str(port),
                peers_count,
                gossip_status,
                cpu,
                mem,
                uptime,
                latency,
            )

        # Store gossip data for selection events
        self.gossip_data = gossip_data
        self.sorted_urls = sorted_urls
        
        # Restore cursor if possible, otherwise select first row
        if cursor_row < table.row_count:
            table.move_cursor(row=cursor_row)
        elif table.row_count > 0:
            table.move_cursor(row=0)
            
        # Auto-update gossip details for the selected row
        if table.row_count > 0:
            current_row = table.cursor_row
            if current_row < len(sorted_urls):
                selected_url = sorted_urls[current_row]
                selected_gossip = gossip_data.get(selected_url, {})
                try:
                    gossip_widget = self.app.query_one(GossipDetails)
                    gossip_widget.update_gossip_details(selected_url, selected_gossip)
                except Exception as e:
                    log(f"Error auto-updating gossip details: {e}")

    def on_data_table_row_selected(self, event) -> None:
        """Handle row selection to show gossip details."""
        if hasattr(self, 'gossip_data') and hasattr(self, 'sorted_urls'):
            if event.cursor_row < len(self.sorted_urls):
                selected_url = self.sorted_urls[event.cursor_row]
                gossip_data = self.gossip_data.get(selected_url, {})
                
                # Update gossip details widget
                try:
                    gossip_widget = self.app.query_one(GossipDetails)
                    gossip_widget.update_gossip_details(selected_url, gossip_data)
                except Exception as e:
                    log(f"Error updating gossip details: {e}")

    def on_data_table_row_highlighted(self, event) -> None:
        """Handle row highlighting to show gossip details immediately."""
        if hasattr(self, 'gossip_data') and hasattr(self, 'sorted_urls'):
            if event.cursor_row < len(self.sorted_urls):
                selected_url = self.sorted_urls[event.cursor_row]
                gossip_data = self.gossip_data.get(selected_url, {})
                
                # Update gossip details widget
                try:
                    gossip_widget = self.app.query_one(GossipDetails)
                    gossip_widget.update_gossip_details(selected_url, gossip_data)
                except Exception as e:
                    log(f"Error updating gossip details: {e}")


class RingSegment(Static):
    """A single segment of the ring visualization."""

    pass


class RingVisualizer(Static):
    """A widget to visualize the ring state and balance."""

    def compose(self) -> ComposeResult:
        yield Label("RING DISTRIBUTION & BALANCE", classes="section-title")
        yield Container(id="ring_bar_container")
        yield Label("Partition Ownership", classes="subtitle")
        yield CyclicDataTable(id="ring_legend")
        yield Label("", id="balance_stats")

    def on_mount(self) -> None:
        table = self.query_one(CyclicDataTable)
        table.add_columns("NODE", "TOKEN SHARE", "RANGE COUNT", "DEVIATION")
        table.cursor_type = "row"

    def update_ring(self, ring_data: dict) -> None:
        bar_container = self.query_one("#ring_bar_container")
        table = self.query_one(CyclicDataTable)
        stats_label = self.query_one("#balance_stats")

        # Clear previous state
        bar_container.remove_children()
        table.clear()

        if "error" in ring_data:
            bar_container.mount(Label(f"[{ERROR_RED}]Error: {ring_data['error']}[/]"))
            stats_label.update("")
            return

        ranges_map = ring_data.get("ranges", {})
        parsed_ranges = []
        total_tokens = 0

        # Parse data
        for node_url, range_list in ranges_map.items():
            node_total_size = 0
            for range_info in range_list:
                try:
                    size = int(range_info.get("size", 0))
                    node_total_size += size
                    total_tokens += size
                except (ValueError, TypeError):
                    continue
            if node_total_size > 0:
                parsed_ranges.append((node_url, node_total_size))

        # Sort by node URL
        parsed_ranges.sort(key=lambda x: x[0])

        # Calculate Statistics
        sizes = [s for _, s in parsed_ranges]
        if sizes:
            mean_size = statistics.mean(sizes)
            try:
                stdev = statistics.stdev(sizes)
            except statistics.StatisticsError:
                stdev = 0
            cv = (stdev / mean_size) * 100 if mean_size > 0 else 0
            stats_label.update(
                f"Mean Share: {mean_size:.2e} | Std Dev: {stdev:.2e} | CV: {cv:.1f}%"
            )
        else:
            mean_size = 0
            stats_label.update("No Data")

        # Colors for nodes
        colors = ["#84cc16", "#3b82f6", "#a855f7", "#06b6d4", "#eab308", "#ef4444"]

        if total_tokens > 0:
            for i, (node_url, size) in enumerate(parsed_ranges):
                share = size / total_tokens
                color = colors[i % len(colors)]

                # Deviation
                dev = size - mean_size
                pct_dev = (dev / mean_size) * 100 if mean_size > 0 else 0
                dev_str = f"{pct_dev:+.1f}%"
                if abs(pct_dev) < 10:
                    dev_colored = f"[green]{dev_str}[/]"
                elif abs(pct_dev) < 20:
                    dev_colored = f"[yellow]{dev_str}[/]"
                else:
                    dev_colored = f"[red]{dev_str}[/]"

                # Create visual segment
                segment = RingSegment()
                segment.styles.background = color
                segment.styles.width = f"{share:.2%}"
                segment.tooltip = f"{node_url}\nShare: {share:.1%}\nDev: {dev_str}"
                bar_container.mount(segment)

                # Add to legend
                node_name = node_url.split(":")[-1]
                table.add_row(
                    f"[{color}]● {node_name}[/]", f"{share:.1%}", f"{size}", dev_colored
                )
        else:
            bar_container.mount(Label("[yellow]Ring is empty or initializing...[/]"))


class DataExplorer(Static):
    """A widget to interact with the KV store."""

    def compose(self) -> ComposeResult:
        yield Label("DATA EXPLORER", classes="section-title")
        yield Container(
            Input(placeholder="Key", id="key_input"),
            Input(placeholder="Value (for SET)", id="value_input"),
            Horizontal(
                Button("GET", variant="primary", id="btn_get"),
                Button("SET", variant="success", id="btn_set"),
                Button("DELETE", variant="error", id="btn_del"),
                classes="button-row",
            ),
            classes="input-container",
        )
        yield Label("Response", classes="subtitle")
        yield Pretty("", id="response_output")

    async def on_button_pressed(self, event: Button.Pressed) -> None:
        key = self.query_one("#key_input").value
        value = self.query_one("#value_input").value
        output = self.query_one("#response_output")
        client = self.app.client  # Access client from App

        if not key:
            output.update({"error": "Key is required"})
            return

        output.update("Requesting...")

        if event.button.id == "btn_get":
            resp = await client.get_key(key)
            output.update(resp)
        elif event.button.id == "btn_set":
            resp = await client.set_key(key, value)
            output.update(resp)
        elif event.button.id == "btn_del":
            resp = await client.delete_key(key)
            output.update(resp)


class LimeDB(App):
    """A Textual app to manage LimeDB Cluster."""

    def __init__(self, host_urls, **kwargs):
        super().__init__(**kwargs)
        if not host_urls:
            raise ValueError("host_urls cannot be empty")
        self.host_urls = host_urls

    CSS = """
    Screen {
        layout: vertical;
        background: #121212;
    }
    
    /* Header & Logo */
    Logo {
        height: 8;
        width: auto;
        margin-bottom: 1;
        align-horizontal: center;
    }
    
    .section-title {
        background: #3f6212;
        color: #ffffff;
        text-style: bold;
        padding: 0 1;
        width: 100%;
    }
    
    .subtitle {
        text-align: center;
        color: #84cc16;
        margin-top: 1;
        text-style: bold;
    }

    /* Layout Containers */
    #main_grid {
        layout: grid;
        grid-size: 3;
        grid-columns: 1fr 1fr 1fr;
        grid-gutter: 1;
        padding: 1;
    }
    
    #status_container, #ring_container, #gossip_container {
        height: 100%;
        border: solid #3f6212;
        background: #1a1a1a;
    }
    
    .gossip-container {
        padding: 1;
    }
    
    #gossip_info {
        margin-bottom: 1;
        padding: 1;
        background: #262626;
        border: solid #3f6212;
    }
    
    /* Ring Bar */
    #ring_bar_container {
        height: 3;
        width: 100%;
        layout: horizontal;
        background: #262626;
        margin: 1 0;
    }
    
    RingSegment {
        height: 100%;
    }
    
    /* Data Tables */
    CyclicDataTable {
        background: #1a1a1a;
        border: none;
    }
    
    CyclicDataTable > .datatable--header {
        background: #262626;
        color: #84cc16;
        text-style: bold;
    }
    
    CyclicDataTable > .datatable--cursor {
        background: #3f6212;
        color: #ffffff;
    }
    
    /* Data Explorer */
    .input-container {
        padding: 1;
        border: solid #3f6212;
        margin-bottom: 1;
    }
    
    Input {
        margin-bottom: 1;
        border: solid #84cc16;
    }
    
    .button-row {
        height: auto;
        align: center middle;
    }
    
    Button {
        margin: 0 1;
    }
    
    #response_output {
        border: solid #3f6212;
        padding: 1;
        height: 1fr;
        background: #1a1a1a;
    }
    
    /* Tabs */
    TabbedContent {
        height: 1fr;
    }
    
    TabPane {
        padding: 0;
    }
    
    Tab {
        /* cursor not supported */
    }
    
    #balance_stats {
        text-align: center;
        color: #84cc16;
        padding: 1;
    }
    """

    BINDINGS = [("q", "quit", "Quit"), ("r", "refresh", "Refresh Now")]

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        yield Logo()

        with TabbedContent():
            with TabPane("Overview", id="tab_overview"):
                yield Grid(
                    Container(ClusterStatus(), id="status_container"),
                    Container(RingVisualizer(), id="ring_container"),
                    Container(GossipDetails(), id="gossip_container"),
                    id="main_grid",
                )
            with TabPane("Data Explorer", id="tab_explorer"):
                yield DataExplorer()

        yield Footer()

    def on_mount(self) -> None:
        self.client = ClusterClient(self.host_urls)
        self.set_interval(2.0, self.update_data)
        self.call_later(self.update_data)

    async def update_data(self) -> None:
        status_data = await self.client.get_all_nodes_status()

        ring_data = {"error": "No active nodes"}
        for url, data in status_data.items():
            if "error" not in data:
                ring_data = await self.client.get_ring_state(url)
                break

        # Fetch Metrics and Gossip Data (Parallel)
        metrics_data = {}
        gossip_data = {}
        tasks = []
        urls = sorted(status_data.keys())

        for url in urls:
            if "error" not in status_data[url]:
                tasks.append(self.fetch_node_metrics(url))
                tasks.append(self.client.get_gossip_metrics(url))
            else:
                metrics_data[url] = {"error": "Node Down"}
                gossip_data[url] = {"error": "Node Down"}

        results = await asyncio.gather(*tasks)
        active_urls = [u for u in urls if "error" not in status_data[u]]

        # Split results between metrics and gossip (alternating in tasks list)
        for i, url in enumerate(active_urls):
            metrics_data[url] = results[i * 2]      # metrics
            gossip_data[url] = results[i * 2 + 1]   # gossip

        self.query_one(ClusterStatus).update_status(status_data, metrics_data, gossip_data)
        self.query_one(RingVisualizer).update_ring(ring_data)

    async def fetch_node_metrics(self, base_url: str) -> dict:
        metrics = await self.client.get_metrics(base_url)
        latency = await self.client.measure_latency(base_url)
        metrics["latency"] = latency
        return metrics

    def action_refresh(self) -> None:
        self.call_later(self.update_data)


def parse_args():
    parser = argparse.ArgumentParser(
        description="LimeDB TUI - Terminal User Interface for LimeDB Cluster",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python main.py --hosts http://192.168.1.125:8484,http://192.168.1.126:8484,http://192.168.1.127:8484
  python main.py --hosts http://localhost:8484,http://localhost:8485,http://localhost:8486
        """,
    )

    parser.add_argument(
        "--hosts",
        type=str,
        required=True,
        help="Comma-separated list of host URLs (REQUIRED)",
    )

    return parser.parse_args()


if __name__ == "__main__":
    try:
        args = parse_args()

        # Parse the comma-separated host URLs
        host_urls = [url.strip() for url in args.hosts.split(",") if url.strip()]

        if not host_urls:
            log("Error: No valid host URLs provided after parsing")
            log("Please provide valid URLs separated by commas")
            sys.exit(1)

        log(f"Connecting to LimeDB cluster hosts: {', '.join(host_urls)}")

        app = LimeDB(host_urls=host_urls)
        app.run()

    except SystemExit as e:
        # This catches argparse exits (like --help or missing required args)
        if e.code != 0:
            log("\nUsage: python main.py --hosts <comma-separated-urls>")
            log(
                "Example: python main.py --hosts http://192.168.1.125:8484,http://192.168.1.126:8484"
            )
        sys.exit(e.code)
    except Exception as e:
        log(f"Error: {e}")
        sys.exit(1)
