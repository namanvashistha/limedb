from textual.app import App, ComposeResult
from textual.widgets import Header, Footer, Static, DataTable, Label, TabbedContent, TabPane, Input, Button, Pretty
from textual.containers import Container, Vertical, Horizontal, Grid
from textual.reactive import reactive
from textual.binding import Binding
from client import ClusterClient
import asyncio

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

class ClusterStatus(Static):
    """A widget to display the status of all nodes."""
    
    def compose(self) -> ComposeResult:
        yield Label("CLUSTER NODES", classes="section-title")
        yield CyclicDataTable(id="status_table")

    def on_mount(self) -> None:
        table = self.query_one(CyclicDataTable)
        table.add_columns("STATUS", "NODE ID", "PORT", "PEERS")
        table.cursor_type = "row"
        table.zebra_stripes = False
        table.show_header = True

    def update_status(self, status_data: dict) -> None:
        table = self.query_one(CyclicDataTable)
        # Save cursor position
        cursor_row = table.cursor_row
        table.clear()
        
        sorted_ports = sorted(status_data.keys())
        
        for port in sorted_ports:
            data = status_data[port]
            if "error" in data:
                status_icon = f"[{ERROR_RED}]● DOWN[/]"
                table.add_row(status_icon, "?", str(port), str(data["error"]))
            else:
                node_id = str(data.get("nodeId", "?"))
                status = data.get("status", "UNKNOWN")
                peers_count = len(data.get("peers", []))
                
                if status == "active":
                    status_icon = f"[{LIME_GREEN}]● ACTIVE[/]"
                else:
                    status_icon = f"[{ERROR_RED}]● {status.upper()}[/]"
                
                table.add_row(status_icon, f"[bold white]{node_id}[/]", str(port), str(peers_count))
        
        # Restore cursor if possible
        if cursor_row < table.row_count:
            table.move_cursor(row=cursor_row)

class RingSegment(Static):
    """A single segment of the ring visualization."""
    pass

class RingVisualizer(Static):
    """A widget to visualize the ring state as a segmented bar."""

    def compose(self) -> ComposeResult:
        yield Label("RING DISTRIBUTION", classes="section-title")
        yield Container(id="ring_bar_container")
        yield Label("Partition Ownership", classes="subtitle")
        yield CyclicDataTable(id="ring_legend")

    def on_mount(self) -> None:
        table = self.query_one(CyclicDataTable)
        table.add_columns("NODE", "TOKEN SHARE", "RANGE COUNT")
        table.cursor_type = "row"

    def update_ring(self, ring_data: dict) -> None:
        bar_container = self.query_one("#ring_bar_container")
        table = self.query_one(CyclicDataTable)
        
        # Clear previous state
        bar_container.remove_children()
        table.clear()
        
        if "error" in ring_data:
            bar_container.mount(Label(f"[{ERROR_RED}]Error: {ring_data['error']}[/]"))
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
        
        # Sort by node URL for consistent coloring
        parsed_ranges.sort(key=lambda x: x[0])
        
        # Colors for nodes
        colors = ["#84cc16", "#3b82f6", "#a855f7", "#06b6d4", "#eab308", "#ef4444"]
        
        if total_tokens > 0:
            for i, (node_url, size) in enumerate(parsed_ranges):
                share = size / total_tokens
                color = colors[i % len(colors)]
                
                # Create visual segment
                segment = RingSegment()
                segment.styles.background = color
                segment.styles.width = f"{share:.2%}"
                segment.tooltip = f"{node_url}\nShare: {share:.1%}"
                bar_container.mount(segment)
                
                # Add to legend
                node_name = node_url.split(":")[-1]
                table.add_row(f"[{color}]● {node_name}[/]", f"{share:.1%}", f"{size}")
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
                classes="button-row"
            ),
            classes="input-container"
        )
        yield Label("Response", classes="subtitle")
        yield Pretty("", id="response_output")

    async def on_button_pressed(self, event: Button.Pressed) -> None:
        key = self.query_one("#key_input").value
        value = self.query_one("#value_input").value
        output = self.query_one("#response_output")
        client = self.app.client # Access client from App
        
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

class LimeDBTui(App):
    """A Textual app to manage LimeDB Cluster."""

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
        grid-size: 2;
        grid-columns: 1fr 1fr;
        grid-gutter: 1;
        padding: 1;
    }
    
    #status_container, #ring_container {
        height: 100%;
        border: solid #3f6212;
        background: #1a1a1a;
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
                    id="main_grid"
                )
            with TabPane("Data Explorer", id="tab_explorer"):
                yield DataExplorer()
                
        yield Footer()

    def on_mount(self) -> None:
        self.client = ClusterClient()
        self.set_interval(2.0, self.update_data)
        self.call_later(self.update_data)

    async def update_data(self) -> None:
        # Only update overview if it's visible (optimization)
        # For now, just update always to keep it simple
        status_data = await self.client.get_all_nodes_status()
        
        ring_data = {"error": "No active nodes"}
        for port, data in status_data.items():
            if "error" not in data:
                ring_data = await self.client.get_ring_state(port)
                break
        
        self.query_one(ClusterStatus).update_status(status_data)
        self.query_one(RingVisualizer).update_ring(ring_data)

    def action_refresh(self) -> None:
        self.call_later(self.update_data)

if __name__ == "__main__":
    app = LimeDBTui()
    app.run()
