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
    ListView,
    ListItem,
    LoadingIndicator,
)
from textual.containers import Container, Horizontal, Grid, Vertical
from textual.binding import Binding
from textual.screen import ModalScreen
from client import ClusterClient
from textual import log
import asyncio
import statistics
import sys
import argparse
import json
from datetime import datetime

# Theme Colors
LIME_GREEN = "#84cc16"
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


class HelpModal(ModalScreen):
    """A modal screen to show help."""

    def compose(self) -> ComposeResult:
        with Container(id="help_container"):
            yield Label("LimeDB TUI Help", classes="section-title")
            yield Label("\nNavigation:", classes="subtitle")
            yield Label("  1       : Switch to Overview Tab")
            yield Label("  2       : Switch to Data Explorer Tab")
            yield Label("  3       : Switch to Events Tab")
            yield Label("  q       : Quit")
            yield Label("  r       : Force Refresh")
            yield Label("  h       : Toggle this Help")
            yield Label("\nInteraction:", classes="subtitle")
            yield Label("  Enter   : Inspect Node (in Overview)")
            yield Label("  Click rows in tables to see details.")
            yield Button("Close", variant="primary", id="close_help")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        self.dismiss()


class NodeDetailsModal(ModalScreen):
    """A modal screen to inspect node details."""

    def __init__(self, node_url: str, data: dict):
        super().__init__()
        self.node_url = node_url
        self.data = data

    def compose(self) -> ComposeResult:
        with Container(id="node_inspector_container"):
            yield Label(f"Node Inspector: {self.node_url}", classes="section-title")
            yield Pretty(self.data, id="inspector_content")
            yield Button("Close", variant="primary", id="close_inspector")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        self.dismiss()


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


class SummaryCard(Static):
    """A simple summary card."""
    
    def __init__(self, label: str, value: str, **kwargs):
        super().__init__(**kwargs)
        self.label_text = label
        self.value_text = value

    def compose(self) -> ComposeResult:
        yield Label(f"{self.label_text}: {self.value_text}", classes="summary-text")

    def update_value(self, value: str) -> None:
        self.query_one(Label).update(f"[bold]{self.label_text}:[/] {value}")


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
            info_label.update(f"[{ERROR_RED}]No gossip data available for {node_url}[/]")
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
            health_colored = f"[{ERROR_RED}]✗ {cluster_health.upper()}[/]"
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
                status_colored = f"[{ERROR_RED}]● {status.upper()}[/]"
            else:
                status_colored = f"[dim]● {status.upper()}[/]"

            # Lag color coding
            if lag == 0:
                lag_colored = f"[green]{lag}[/]"
            elif lag <= 5:
                lag_colored = f"[yellow]{lag}[/]"
            else:
                lag_colored = f"[{ERROR_RED}]{lag}[/]"

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
        yield Input(placeholder="Filter nodes...", id="filter_input")
        yield CyclicDataTable(id="status_table")

    def on_mount(self) -> None:
        table = self.query_one(CyclicDataTable)
        table.add_columns(
            "STATUS", "NODE URL", "PORT", "PEERS", "GOSSIP", "CPU", "MEM", "UPTIME", "LATENCY"
        )
        table.cursor_type = "row"
        table.zebra_stripes = False
        table.show_header = True

    def on_input_changed(self, event: Input.Changed) -> None:
        """Refresh table when filter changes."""
        if hasattr(self, 'status_data'):
            self.update_status(self.status_data, self.metrics_data, self.gossip_data)

    def update_status(self, status_data: dict, metrics_data: dict, gossip_data: dict) -> None:
        table = self.query_one(CyclicDataTable)
        filter_text = self.query_one("#filter_input").value.lower()
        
        # Save cursor position
        cursor_row = table.cursor_row
        table.clear()

        sorted_urls = sorted(status_data.keys())
        filtered_urls = []

        for url in sorted_urls:
            # Filter logic
            if filter_text and filter_text not in url.lower():
                continue
            
            filtered_urls.append(url)
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
            except Exception:
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
                    gossip_status = f"[{ERROR_RED}]✗ {active_peers}/{total_peers}[/]"
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
                    latency = f"[{ERROR_RED}]{latency_str}[/]"

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

        # Store data for selection events
        self.gossip_data = gossip_data
        self.status_data = status_data
        self.metrics_data = metrics_data
        self.sorted_urls = filtered_urls
        
        # Restore cursor if possible, otherwise select first row
        if cursor_row < table.row_count:
            table.move_cursor(row=cursor_row)
        elif table.row_count > 0:
            table.move_cursor(row=0)
            
        # Auto-update gossip details for the selected row
        if table.row_count > 0:
            current_row = table.cursor_row
            if current_row < len(filtered_urls):
                selected_url = filtered_urls[current_row]
                selected_gossip = gossip_data.get(selected_url, {})
                try:
                    gossip_widget = self.app.query_one(GossipDetails)
                    gossip_widget.update_gossip_details(selected_url, selected_gossip)
                except Exception:
                    pass

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
                except Exception:
                    pass

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
                except Exception:
                    pass


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
                    dev_colored = f"[{ERROR_RED}]{dev_str}[/]"

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
        with Container(id="explorer_wrapper"):
            with Container(id="explorer_main"):
                with Container(classes="input-group"):
                    yield Input(placeholder="Key", id="key_input")
                    yield Input(placeholder="Value (for SET)", id="value_input")
                    with Horizontal(classes="button-row"):
                        yield Button("GET", variant="primary", id="btn_get")
                        yield Button("SET", variant="success", id="btn_set")
                        yield Button("DELETE", variant="error", id="btn_del")
                
                yield Label("Response", classes="subtitle")
                yield Container(
                    Pretty("", id="response_output"),
                    LoadingIndicator(id="loading_indicator"),
                    id="response_container"
                )
            
            with Vertical(id="history_sidebar"):
                yield Label("History", classes="subtitle")
                yield ListView(id="history_list")

    def on_mount(self) -> None:
        self.query_one("#loading_indicator").display = False

    async def on_button_pressed(self, event: Button.Pressed) -> None:
        key = self.query_one("#key_input").value
        value = self.query_one("#value_input").value
        output = self.query_one("#response_output")
        loading = self.query_one("#loading_indicator")
        history_list = self.query_one("#history_list")
        client = self.app.client  # Access client from App

        if not key:
            self.app.notify("Key is required", severity="error")
            return

        # Show loading, hide output
        output.display = False
        loading.display = True
        
        # Add to history
        action = "UNKNOWN"
        if event.button.id == "btn_get":
            action = "GET"
            resp = await client.get_key(key)
        elif event.button.id == "btn_set":
            action = "SET"
            resp = await client.set_key(key, value)
            if "error" not in resp:
                self.app.notify(f"Set {key} successfully", severity="information")
        elif event.button.id == "btn_del":
            action = "DEL"
            resp = await client.delete_key(key)
            if "error" not in resp:
                self.app.notify(f"Deleted {key}", severity="warning")
        
        # Hide loading, show output
        loading.display = False
        output.display = True

        # Try to parse JSON for pretty printing
        try:
            if isinstance(resp, dict) and "body" in resp:
                body_json = json.loads(resp["body"])
                resp["body"] = body_json
        except Exception:
            pass

        output.update(resp)
        
        if "error" in resp:
            self.app.notify(f"Error: {resp['error']}", severity="error")
        
        # Add history item
        timestamp = datetime.now().strftime("%H:%M:%S")
        history_item = ListItem(Label(f"[{timestamp}] {action} {key}"))
        history_item.action = action
        history_item.key = key
        history_item.value = value
        history_list.append(history_item)
        history_list.index = len(history_list.children) - 1

    def on_list_view_selected(self, event: ListView.Selected) -> None:
        """Populate inputs from history."""
        item = event.item
        if hasattr(item, 'key'):
            self.query_one("#key_input").value = item.key
            self.query_one("#value_input").value = item.value if item.value else ""


class EventLog(Static):
    """A widget to display cluster events."""

    def compose(self) -> ComposeResult:
        yield Label("CLUSTER EVENT LOG", classes="section-title")
        yield DataTable(id="events_table")

    def on_mount(self) -> None:
        table = self.query_one(DataTable)
        table.add_columns("TIME", "EVENT TYPE", "DETAILS")
        table.cursor_type = "row"
        table.zebra_stripes = True

    def add_event(self, event_type: str, details: str) -> None:
        table = self.query_one(DataTable)
        timestamp = datetime.now().strftime("%H:%M:%S")
        
        # Color code event types
        if "DOWN" in event_type or "CRITICAL" in event_type:
            type_colored = f"[{ERROR_RED}]{event_type}[/]"
        elif "JOIN" in event_type or "HEALTHY" in event_type:
            type_colored = f"[{LIME_GREEN}]{event_type}[/]"
        else:
            type_colored = f"[yellow]{event_type}[/]"
            
        table.add_row(timestamp, type_colored, details)
        table.move_cursor(row=table.row_count - 1)


class LimeDB(App):
    """A Textual app to manage LimeDB Cluster."""

    CSS_PATH = "styles.tcss"
    
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("r", "refresh", "Refresh Now"),
        Binding("1", "switch_tab('tab_overview')", "Overview"),
        Binding("2", "switch_tab('tab_explorer')", "Data Explorer"),
        Binding("3", "switch_tab('tab_events')", "Events"),
        Binding("h", "toggle_help", "Help"),
        Binding("enter", "inspect_node", "Inspect Node"),
    ]

    def __init__(self, host_urls, **kwargs):
        super().__init__(**kwargs)
        if not host_urls:
            raise ValueError("host_urls cannot be empty")
        self.host_urls = host_urls
        self.previous_status_data = {}

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        yield Logo()

        with TabbedContent(initial="tab_overview"):
            with TabPane("Overview", id="tab_overview"):
                # Summary Cards
                with Horizontal(id="summary_row"):
                    yield SummaryCard("Cluster Health", "Unknown", id="card_health", classes="summary-card")
                    yield SummaryCard("Active Nodes", "0", id="card_nodes", classes="summary-card")
                    yield SummaryCard("Total Keys", "Unknown", id="card_keys", classes="summary-card")

                # Main Grid
                with Container(id="main_grid"):
                    # Left Panel: Status and Ring
                    with Vertical(id="left_panel"):
                        yield Container(ClusterStatus(), id="status_container", classes="card")
                        yield Container(RingVisualizer(), id="ring_container", classes="card")
                    
                    # Right Panel: Gossip Details
                    with Vertical(id="right_panel"):
                        yield Container(GossipDetails(), id="gossip_container", classes="card")

            with TabPane("Data Explorer", id="tab_explorer"):
                yield DataExplorer()
                
            with TabPane("Events", id="tab_events"):
                yield EventLog()

        yield Footer()

    def on_mount(self) -> None:
        self.client = ClusterClient(self.host_urls)
        self.set_interval(2.0, self.update_data)
        self.call_later(self.update_data)

    def action_switch_tab(self, tab_id: str) -> None:
        self.query_one(TabbedContent).active = tab_id

    def action_toggle_help(self) -> None:
        self.push_screen(HelpModal())

    def action_inspect_node(self) -> None:
        """Inspect the currently selected node in the status table."""
        # Only works if we are on the overview tab
        if self.query_one(TabbedContent).active != "tab_overview":
            return
            
        status_widget = self.query_one(ClusterStatus)
        table = status_widget.query_one(CyclicDataTable)
        
        if hasattr(status_widget, 'sorted_urls') and table.cursor_row < len(status_widget.sorted_urls):
            url = status_widget.sorted_urls[table.cursor_row]
            
            # Aggregate all data we have for this node
            node_data = {
                "status": status_widget.status_data.get(url, {}),
                "metrics": status_widget.metrics_data.get(url, {}),
                "gossip": status_widget.gossip_data.get(url, {})
            }
            
            self.push_screen(NodeDetailsModal(url, node_data))

    async def update_data(self) -> None:
        # Discover hosts via gossip from any known seed
        await self.discover_cluster_hosts()

        # Build status_data purely from gossip (deprecated /cluster/state removed)
        status_data = {}
        gossip_data = {}
        metrics_data = {}
        tasks = []
        urls = sorted(self.client.base_urls)

        for url in urls:
            tasks.append(self.client.get_gossip_metrics(url))  # gossip
            tasks.append(self.fetch_node_metrics(url))          # metrics

        results = await asyncio.gather(*tasks, return_exceptions=True)

        # Pair results (gossip, metrics)
        for idx, url in enumerate(urls):
            gossip_result = results[idx * 2]
            metrics_result = results[idx * 2 + 1]

            if isinstance(gossip_result, Exception):
                gossip_data[url] = {"error": str(gossip_result)}
            else:
                gossip_data[url] = gossip_result

            if isinstance(metrics_result, Exception):
                metrics_data[url] = {"error": str(metrics_result)}
            else:
                metrics_data[url] = metrics_result

            # Derive status
            if "error" in gossip_data[url]:
                status_data[url] = {"error": gossip_data[url]["error"]}
            else:
                cluster_health = gossip_data[url].get("cluster_health", "unknown")
                peer_details = gossip_data[url].get("peer_details", [])
                active_peer_urls = [p.get("url") for p in peer_details if p.get("status") == "active"]
                status_data[url] = {
                    "nodeUrl": url,
                    "status": "active",  # If reachable via gossip metrics
                    "peers": active_peer_urls,
                    "cluster_health": cluster_health,
                }

        # Events based on derived status
        self.check_for_events(status_data)
        self.previous_status_data = status_data

        # Ring data from any active node
        ring_data = {"error": "No active nodes"}
        for url in urls:
            if "error" not in status_data[url]:
                ring_candidate = await self.client.get_ring_state(url)
                ring_data = ring_candidate
                break

        self.query_one(ClusterStatus).update_status(status_data, metrics_data, gossip_data)
        self.query_one(RingVisualizer).update_ring(ring_data)
        
        # Update Summary Cards
        active_nodes = len([u for u in status_data if "error" not in status_data[u]])
        self.query_one("#card_nodes").update_value(f"[green]{active_nodes}[/]")
        
        # Determine overall health
        health = "Healthy"
        if active_nodes < len(status_data):
            health = "Degraded"
        if active_nodes == 0:
            health = "Critical"
        
        color = "green" if health == "Healthy" else "yellow" if health == "Degraded" else "red"
        self.query_one("#card_health").update_value(f"[{color}]{health}[/]")
        
        # Keys count
        total_keys = 0
        ranges_container = ring_data.get("ranges")
        if isinstance(ranges_container, dict):
            for ranges in ranges_container.values():
                for r in ranges:
                    if isinstance(r, dict):
                        total_keys += r.get("size", 0)
        self.query_one("#card_keys").update_value(f"[blue]{total_keys}[/]")

    def check_for_events(self, current_status: dict) -> None:
        """Compare current status with previous to generate events."""
        if not self.previous_status_data:
            return

        event_log = self.query_one(EventLog)
        
        # Check for node status changes
        all_urls = set(current_status.keys()) | set(self.previous_status_data.keys())
        
        for url in all_urls:
            prev = self.previous_status_data.get(url, {})
            curr = current_status.get(url, {})
            
            prev_err = "error" in prev
            curr_err = "error" in curr
            
            if not prev and curr:
                event_log.add_event("NODE JOIN", f"Node {url} discovered")
                self.notify(f"Node {url} joined the cluster", severity="information")
            elif prev and not curr:
                event_log.add_event("NODE LEFT", f"Node {url} lost")
                self.notify(f"Node {url} left the cluster", severity="warning")
            elif prev_err and not curr_err:
                event_log.add_event("NODE UP", f"Node {url} is back online")
                self.notify(f"Node {url} is back online", severity="information")
            elif not prev_err and curr_err:
                event_log.add_event("NODE DOWN", f"Node {url} is unreachable")
                self.notify(f"Node {url} is unreachable", severity="error")

    async def discover_cluster_hosts(self) -> None:
        """Discover all cluster hosts through gossip protocol."""
        # Try to get gossip data from any available host
        for url in self.client.base_urls.copy():  # Use copy to avoid modifying during iteration
            try:
                gossip_data = await self.client.get_gossip_metrics(url)
                if "error" not in gossip_data:
                    # Update discovered hosts based on gossip data
                    self.client.update_discovered_hosts(gossip_data, url)
                    break
            except Exception:
                continue

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
  python main.py --seed http://192.168.1.125:8484
  python main.py --seed http://localhost:7001,http://localhost:7002
        """,
    )

    parser.add_argument(
        "--seed",
        type=str,
        required=True,
        help="Comma-separated list of seed host URLs for cluster discovery",
    )

    return parser.parse_args()


if __name__ == "__main__":
    try:
        args = parse_args()

        # Parse seed hosts
        host_urls = [url.strip() for url in args.seed.split(",") if url.strip()]

        if not host_urls:
            log("Error: No valid seed URLs provided after parsing")
            log("Please provide valid URLs separated by commas")
            sys.exit(1)

        log(f"Connecting to LimeDB cluster via seed hosts: {', '.join(host_urls)}")
        log("TUI will automatically discover all cluster nodes through gossip protocol")

        app = LimeDB(host_urls=host_urls)
        app.run()

    except SystemExit as e:
        # This catches argparse exits (like --help or missing required args)
        if e.code != 0:
            log("\nUsage: python main.py --seed <comma-separated-seed-urls>")
            log("Example: python main.py --seed http://localhost:7001")
            log("         python main.py --seed http://192.168.1.125:8484,http://192.168.1.126:8484")
        sys.exit(e.code)
    except Exception:
        sys.exit(1)
