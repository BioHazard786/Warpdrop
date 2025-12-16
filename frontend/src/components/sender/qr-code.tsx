"use client";

import QRCodeStyling, { type Options } from "qr-code-styling";
import { useEffect, useRef } from "react";
import { useIsMobile } from "@/hooks/use-is-mobile";
import { createSharableLink } from "@/lib/utils";

const BASE64_LOGO =
	"data:image/png;base64,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";

export default function QRCode({
	roomId,
	width: w,
	height: h,
}: {
	roomId: string;
	width?: number;
	height?: number;
}) {
	const ref = useRef<HTMLDivElement>(null);
	const qrCodeRef = useRef<QRCodeStyling | null>(null);
	const isMobile = useIsMobile();
	const width = w || (isMobile ? 200 : 300);
	const height = h || (isMobile ? 200 : 300);

	// Watch for theme changes via MutationObserver on <html> class changes
	useEffect(() => {
		const updateQRCode = () => {
			const options = createQRCodeOptions(roomId, width, height);
			if (qrCodeRef.current) {
				qrCodeRef.current.update(options);
			} else {
				qrCodeRef.current = new QRCodeStyling(options);
				if (ref.current) {
					qrCodeRef.current.append(ref.current);
				}
			}
		};

		// Initial render
		updateQRCode();

		// Observe class/style changes on <html> for theme switches
		const observer = new MutationObserver(() => {
			updateQRCode();
		});

		observer.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ["class"],
		});

		return () => observer.disconnect();
	}, [roomId, width, height]);

	return <div ref={ref} />;
}

function createQRCodeOptions(
	roomId: string,
	width: number,
	height: number,
): Options {
	const primaryColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-primary")
		.trim();

	const backgroundColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-background")
		.trim();

	const accentColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-cyan-600")
		.trim();

	return {
		width: width,
		height: height,
		type: "svg",
		data: createSharableLink(roomId),
		image: BASE64_LOGO,
		margin: 2,
		qrOptions: {
			typeNumber: 0,
			mode: "Byte",
			errorCorrectionLevel: "Q",
		},
		imageOptions: {
			hideBackgroundDots: true,
			imageSize: 0.4,
			margin: 0,
			crossOrigin: "anonymous",
			saveAsBlob: true,
		},
		dotsOptions: {
			type: "dots",
			color: primaryColor,
			roundSize: true,
		},
		cornersDotOptions: {
			type: "dots",
			color: accentColor,
		},
		cornersSquareOptions: { type: "dots", color: accentColor },
		backgroundOptions: {
			color: backgroundColor,
		},
	};
}
