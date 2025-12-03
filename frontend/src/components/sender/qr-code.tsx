"use client";

import QRCodeStyling, { type Options } from "qr-code-styling";
import { useEffect, useRef } from "react";
import { createSharableLink } from "@/lib/utils";

export default function QRCode({
	roomId,
	width,
	height,
}: {
	roomId: string;
	width?: number;
	height?: number;
}) {
	const ref = useRef<HTMLDivElement>(null);
	const qrCodeRef = useRef<QRCodeStyling | null>(null);

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
	width?: number,
	height?: number,
): Options {
	const primaryColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-primary")
		.trim();

	console.log(primaryColor);

	const backgroundColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-background")
		.trim();

	const accentColor = getComputedStyle(document.documentElement)
		.getPropertyValue("--color-cyan-600")
		.trim();

	return {
		width: width || 300,
		height: height || 300,
		type: "svg",
		data: createSharableLink(roomId),
		image: `${window.location.origin}/zap.png`,
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
