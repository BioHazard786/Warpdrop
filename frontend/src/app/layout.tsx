import type { Metadata, Viewport } from "next";
import { AppProvider } from "@/app/provider";
import "./globals.css";
import Footer from "@/components/layouts/footer";

export const metadata: Metadata = {
	title: "WarpDrop - P2P File Transfer",
	description:
		"Instant, private, peer-to-peer file transfer. No limits, no signups, no cloud storage.",
	metadataBase: new URL(`https://${process.env.NEXT_PUBLIC_DOMAIN}`),
	applicationName: "WarpDrop",
	authors: [
		{
			name: "Mohd Zaid",
			url: "https://github.com/BioHazard786",
		},
	],
	creator: "Mohd Zaid",
	publisher: "Mohd Zaid",
	keywords: [
		"p2p",
		"file transfer",
		"webrtc",
		"private",
		"encrypted",
		"share",
		"cli",
	],
	openGraph: {
		title: "WarpDrop - P2P File Transfer",
		description: "Instant, private, peer-to-peer file transfer.",
		type: "website",
		siteName: "WarpDrop",
	},
	twitter: {
		card: "summary_large_image",
		title: "WarpDrop - P2P File Transfer",
		description: "Instant, private, peer-to-peer file transfer.",
	},
	appleWebApp: {
		capable: true,
		statusBarStyle: "black-translucent",
		title: "WarpDrop",
	},
	category: "Productivity",
};

export const viewport: Viewport = {
	themeColor: "#000000",
	width: "device-width",
	initialScale: 1,
	maximumScale: 1,
	userScalable: false, // Often used in PWAs to prevent zooming, purely optional
};

export default function RootLayout({
	children,
}: Readonly<{
	children: React.ReactNode;
}>) {
	return (
		<html lang="en" className="dark" suppressHydrationWarning>
			<body className="antialiased">
				<AppProvider>
					{children}
					<div></div>
					<Footer />
				</AppProvider>
			</body>
		</html>
	);
}
