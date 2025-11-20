import asyncio
import websockets
import sys
import os
import pty
import fcntl
import termios
import struct
import json

async def handler(websocket):
    # Create a pseudo-terminal
    master_fd, slave_fd = pty.openpty()

    # Launch the TUI process attached to the PTY
    process = await asyncio.create_subprocess_exec(
        sys.executable, "main.py",
        stdin=slave_fd,
        stdout=slave_fd,
        stderr=slave_fd,
        preexec_fn=os.setsid  # Create a new session
    )
    
    # Close slave_fd in the parent process as it's now owned by the child
    os.close(slave_fd)

    async def read_from_pty():
        try:
            loop = asyncio.get_running_loop()
            while True:
                # Read raw bytes from the PTY master
                data = await loop.run_in_executor(None, os.read, master_fd, 1024)
                if not data:
                    break
                await websocket.send(data.decode(errors="ignore"))
        except OSError:
            pass  # PTY closed
        except Exception:
            pass
        finally:
            await websocket.close()

    async def read_from_socket():
        try:
            async for message in websocket:
                try:
                    msg = json.loads(message)
                    if msg['type'] == 'input':
                        os.write(master_fd, msg['data'].encode())
                    elif msg['type'] == 'resize':
                        # Set terminal window size
                        rows = msg['rows']
                        cols = msg['cols']
                        winsize = struct.pack("HHHH", rows, cols, 0, 0)
                        fcntl.ioctl(master_fd, termios.TIOCSWINSZ, winsize)
                except (json.JSONDecodeError, KeyError):
                    pass # Ignore malformed messages
        except (ConnectionResetError, websockets.exceptions.ConnectionClosed):
            pass
        except Exception:
            pass
        finally:
            # Terminate process if socket closes
            if process.returncode is None:
                process.terminate()
                try:
                    await process.wait()
                except Exception:
                    pass
            os.close(master_fd)

    await asyncio.gather(
        read_from_pty(),
        read_from_socket()
    )

async def main():
    print("Server running at ws://localhost:8765")
    async with websockets.serve(handler, "0.0.0.0", 8765):
        # Keep running until cancelled
        try:
            await asyncio.Future()
        except asyncio.CancelledError:
            print("\nServer stopping...")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass  # Graceful exit on Ctrl+C