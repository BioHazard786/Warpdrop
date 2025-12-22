import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
	return {
		rules: {
			userAgent: "*",
			allow: "/",
		},
		sitemap: `https://${process.env.NEXT_PUBLIC_DOMAIN}/sitemap.xml`,
	};
}
