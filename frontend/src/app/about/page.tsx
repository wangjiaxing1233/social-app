import type { Metadata } from "next";
import { ArchitectureStep } from "@/components/architecture-step";

export const metadata: Metadata = { title: "Architecture | NetworkAI" };

export default function About() {
  return (
    <main className="mx-auto max-w-4xl space-y-6 px-4 py-8 sm:px-8">
      <h1 className="text-3xl font-semibold">How NetworkAI works</h1>
      <ol className="grid list-none gap-4 sm:grid-cols-2">
        <ArchitectureStep title="Browser">
          React renders the interface and sends requests.
        </ArchitectureStep>
        <ArchitectureStep title="Go API">
          Go handles accounts, post search, and media uploads.
        </ArchitectureStep>
        <ArchitectureStep title="Next.js server">
          The AI route verifies the user and calls OpenAI.
        </ArchitectureStep>
        <ArchitectureStep title="Elasticsearch">
          Elasticsearch indexes user and post records for search.
        </ArchitectureStep>
        <ArchitectureStep title="Cloud Storage">
          Cloud Storage holds uploaded images and videos.
        </ArchitectureStep>
      </ol>
    </main>
  );
}
