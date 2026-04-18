import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // standalone output lets us ship a minimal runtime bundle to the VPS —
  // no npm install on the server, just `node server.js`.
  output: "standalone",
  // Pin trace root to landing/ itself — otherwise Next picks up a user-level
  // lockfile (C:\Users\user\package-lock.json) and nests server.js deep under
  // .next/standalone/<full-windows-path>/landing/ which breaks deploy.
  outputFileTracingRoot: __dirname,
  images: {
    formats: ["image/avif", "image/webp"],
  },
  experimental: {
    optimizePackageImports: ["lucide-react", "framer-motion"],
  },
};

export default nextConfig;
