import type { Metadata } from "next";
import { DM_Sans, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Sidebar } from "@/components/Sidebar";

const dmSans = DM_Sans({
  subsets: ["latin"],
  variable: "--font-dm-sans",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "AgentPulse Dashboard",
  description: "Observability and Operations for LLM Agents",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" className={`${dmSans.variable} ${jetbrainsMono.variable}`}>
      <body className="min-h-screen antialiased">
        <div className="flex min-h-screen">
          <Sidebar />
          <main className="flex-1 overflow-y-auto">
            <div className="mx-auto max-w-7xl px-6 py-8 lg:px-10 lg:py-10">
              {children}
            </div>
          </main>
        </div>
      </body>
    </html>
  );
}
