import { NextRequest, NextResponse } from "next/server";

const DEFAULT_LIMIT = 100;
const MAX_LIMIT = 2000;

function parseSince(raw: string | null): number {
  if (!raw || raw.trim() === "") {
    return 0;
  }

  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error("since must be a non-negative integer");
  }

  return parsed;
}

function parseLimit(raw: string | null): number {
  if (!raw || raw.trim() === "") {
    return DEFAULT_LIMIT;
  }

  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error("limit must be a positive integer");
  }

  return Math.min(parsed, MAX_LIMIT);
}

function normalizeBackendUrl(raw: string | null): URL {
  if (!raw || raw.trim() === "") {
    throw new Error("backendUrl is required");
  }

  let parsed: URL;
  try {
    parsed = new URL(raw.trim());
  } catch {
    throw new Error("backendUrl must be a valid URL");
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("backendUrl must use http or https");
  }

  return parsed;
}

export async function GET(request: NextRequest) {
  try {
    const since = parseSince(request.nextUrl.searchParams.get("since"));
    const limit = parseLimit(request.nextUrl.searchParams.get("limit"));
    const backendBaseUrl = normalizeBackendUrl(
      request.nextUrl.searchParams.get("backendUrl")
    );

    const upstreamUrl = new URL("/v1/logs/events", backendBaseUrl);
    upstreamUrl.searchParams.set("since", String(since));
    upstreamUrl.searchParams.set("limit", String(limit));

    const upstreamResponse = await fetch(upstreamUrl, {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      cache: "no-store",
    });

    const text = await upstreamResponse.text();
    const body = text ? JSON.parse(text) : {};

    if (!upstreamResponse.ok) {
      return NextResponse.json(
        {
          error:
            typeof body.error === "string"
              ? body.error
              : `backend request failed (${upstreamResponse.status})`,
        },
        { status: upstreamResponse.status }
      );
    }

    return NextResponse.json(body);
  } catch (error) {
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "failed to fetch log events",
      },
      { status: 400 }
    );
  }
}
