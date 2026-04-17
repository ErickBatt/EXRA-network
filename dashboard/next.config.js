/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Standalone output: creates .next/standalone/ with only required node_modules.
  // Lets us deploy a pre-built zip — server just runs `node server.js`, no npm needed.
  output: 'standalone',
};

module.exports = nextConfig;
