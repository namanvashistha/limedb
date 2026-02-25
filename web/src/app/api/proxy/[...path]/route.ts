import { NextRequest, NextResponse } from "next/server";

const SEED_URL = process.env.LIMEDB_SEED_URL || "http://node1:8484";

async function proxyRequest(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  
  // Check if a specific node is requested via query param
  const targetNode = req.nextUrl.searchParams.get('node');
  const baseUrl = targetNode || SEED_URL;
  const apiBase = `${baseUrl}/api/v1`;
  
  // Build URL with query parameters (excluding 'node' param used for routing)
  const searchParams = new URLSearchParams(req.nextUrl.searchParams);
  searchParams.delete('node'); // Remove node param from forwarded request
  const queryString = searchParams.toString() ? `?${searchParams.toString()}` : '';
  const url = `${apiBase}/${path.join("/")}${queryString}`;
  
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
