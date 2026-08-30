/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export — output goes to web/out/, which Caddy/Go serves directly.
  output: 'export',
  // All API calls go to /api/* which is forwarded to the Go backend.
  trailingSlash: true,
}

export default nextConfig
