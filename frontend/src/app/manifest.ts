import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
	return {
		name: "WarpDrop - P2P File Transfer",
		short_name: "WarpDrop",
		description: "Instant, private, peer-to-peer file transfer. No limits, no signups.",
		start_url: "/",
		display: "standalone",
		background_color: "#000000",
		theme_color: "#000000",
		icons: [
			{
				src: "/icons/icon.svg",
				sizes: "any",
				type: "image/svg+xml",
			},
		],
	};
}
