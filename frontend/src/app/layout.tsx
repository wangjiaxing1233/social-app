import type { Metadata } from "next";
import "./globals.css";
import Link from "next/link";

export const metadata: Metadata = {
  title: "NetworkAI",
  description: "share your private innovation",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en">
      <body>
        <header className="border-b border-slate-200 bg-white">
          <nav
            aria-label="Main navigation"
            className="mx-auto flex max-w-4xl items-center gap-6 px-4 py-3 sm:px-8"
          >
            <Link href="/" className="font-semibold">
              NetworkAI
            </Link>
            <Link href="/" className="text-sm underline">
              Discover
            </Link>
            <Link href="/about" className="text-sm underline">
              Architecture
            </Link>
          </nav>
        </header>
        {children}
      </body>
    </html>
  );
}
