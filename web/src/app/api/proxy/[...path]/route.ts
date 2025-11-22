import { NextRequest, NextResponse } from "next/server";

const SEED_URL = process.env.LIMEDB_SEED_URL || "http://192.168.1.124:8484";
const API_BASE = `${SEED_URL}/api/v1`;

async function proxyRequest(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  const url = `${API_BASE}/${path.join("/")}`;
  
  try {
    const options: RequestInit = {
      method: req.method,
      headers: {
        "Content-Type": "application/json",
      },
    };

    if (req.method !== "GET" && req.method !== "HEAD") {
      const body = await req.text();
      if (body) {
        options.body = body;
      }
    }

    const response = await fetch(url, options);
    const data = await response.text();

    try {
        // Try to parse JSON if possible
        const json = JSON.parse(data);
        return NextResponse.json(json, { status: response.status });
    } catch {
        // Return text if not JSON
        return new NextResponse(data, { status: response.status });
    }

  } catch (error) {
    console.error(`Proxy error for ${url}:`, error);
    return NextResponse.json({ error: "Failed to connect to LimeDB node" }, { status: 502 });
  }
}

export { proxyRequest as GET, proxyRequest as POST, proxyRequest as DELETE, proxyRequest as PUT };
