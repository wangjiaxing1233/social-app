import OpenAI from "openai";
import { ApiError, searchPosts } from "@/lib/api";
import { runContentAgent } from "@/lib/server/content-agent";

const MAX_BODY_CHARS = 5000;

export async function POST(request: Request) {
  try {
    // 1. Read the token and form input.
    const auth = request.headers.get("authorization") ?? "";
    const token = auth.startsWith("Bearer ") ? auth.slice(7) : "";
    if (!token) {
      return Response.json({ error: "Sign in to use the agent." }, { status: 401 });
    }

    // The coding pad text mentions a 5,000-character limit; enforce it here.
    const body = await request.text();
    if (body.length > MAX_BODY_CHARS) {
      return Response.json({ error: "Your input is too long." }, { status: 413 });
    }
    const { interests = "", goal } = JSON.parse(body || "{}");
    if (typeof interests !== "string" || typeof goal !== "string" || !goal.trim()) {
      return Response.json({ error: "Enter a valid goal and interests." }, { status: 400 });
    }

    // 2. Set a time limit and verify the session through Go.
    const signal = AbortSignal.any([request.signal, AbortSignal.timeout(65_000)]);
    await searchPosts(token, {}, signal);

    // 3. Configure OpenAI and run the agent.
    const apiKey = process.env.OPENAI_API_KEY;
    const model = process.env.OPENAI_AGENT_MODEL;
    if (!apiKey || !model) {
      return Response.json({ error: "Configure server AI settings." }, { status: 503 });
    }

    const client = new OpenAI({ apiKey, maxRetries: 0 });
    const result = await runContentAgent(
      client,
      model,
      { interests: interests.trim(), goal: goal.trim() },
      token,
      signal,
    );
    return Response.json(result);
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return Response.json({ error: "Session expired. Sign in again." }, { status: 401 });
    }
    if (error instanceof SyntaxError) {
      return Response.json({ error: "Enter a valid goal and interests." }, { status: 400 });
    }
    return Response.json({ error: "The agent could not finish. Try again." }, { status: 502 });
  }
}
