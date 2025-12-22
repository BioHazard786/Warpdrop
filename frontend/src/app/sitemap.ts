import type { MetadataRoute } from "next";

export default function sitemap(): MetadataRoute.Sitemap {
	return [
		{
			url: `https://${process.env.NEXT_PUBLIC_DOMAIN}`,
			lastModified: new Date(),
			changeFrequency: "yearly",
			priority: 1,
		},
	];
}
