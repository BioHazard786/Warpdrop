import type { NextConfig } from "next";

const nextConfig: NextConfig = {
	/* config options here */
	reactStrictMode: false,
	// reactCompiler: true,
	output: "standalone",
	allowedDevOrigins: ["localhost.biohazard.qzz.io"],
};

export default nextConfig;
