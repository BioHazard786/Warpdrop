"use client";

import Link from "next/link";
import { useEffect, useRef } from "react";
import Github from "@/components/icons/github";
import Heart from "@/components/icons/heart";
import Telegram from "@/components/icons/telegram";
import X from "@/components/icons/x";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { ThemeSwitcher } from "@/components/ui/theme-switcher";

function Footer() {
	const footerRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		if (!footerRef.current) return;

		const update = () => {
			document.documentElement.style.setProperty(
				"--footer-h",
				`${footerRef.current?.offsetHeight}px`,
			);
		};

		update();
		const ro = new ResizeObserver(update);
		ro.observe(footerRef.current);

		return () => ro.disconnect();
	}, []);

	return (
		<footer
			ref={footerRef}
			className="fixed bottom-0 z-100 flex flex-col md:flex-row w-full items-center justify-between bg-[#f9f9f9] dark:bg-[#101012] p-4 py-2.5 border-t gap-4"
		>
			<div className="order-2 md:order-1 flex justify-between items-center w-full md:contents">
				<div className="flex flex-col items-stretch justify-start gap-2 flex-initial text-muted-foreground order-2 md:order-1">
					<p className="text-xs">© {new Date().getFullYear()} Warpdrop, Inc.</p>
					<div className="flex flex-row items-center justify-start gap-3 flex-initial h-5">
						<Link
							href="https://github.com/BioHazard786/Warpdrop"
							target="_blank"
							rel="noreferrer"
						>
							<Github className="size-4 hover:text-muted-foreground/50 text-muted-foreground transition-colors" />
						</Link>
						<Separator orientation="vertical" />
						<Link
							href="https://x.com/Coder_Zaid"
							target="_blank"
							rel="noreferrer"
						>
							<X className="size-4 hover:text-muted-foreground/50 text-muted-foreground transition-colors" />
						</Link>
						<Separator orientation="vertical" />
						<Link
							href="https://telegram.dog/lulu786"
							target="_blank"
							rel="noreferrer"
						>
							<Telegram className="size-4 hover:text-muted-foreground/50 text-muted-foreground transition-colors" />
						</Link>
					</div>
				</div>
				<ThemeSwitcher className="order-3" />
			</div>
			<div className="space-y-1 order-1 md:order-2">
				<div className="flex gap-2 items-center justify-center">
					<p className="text-muted-foreground text-xs">
						<strong>Like Warpdrop?</strong> Support its development!
					</p>
					<Badge variant="sending" asChild>
						<Link
							href="https://github.com/sponsors/BioHazard786"
							target="_blank"
							rel="noreferrer"
						>
							Donate
						</Link>
					</Badge>
				</div>
				<div className="text-xs text-muted-foreground text-center flex items-end justify-center gap-1">
					Cooked with <Heart className="text-red-400 size-3.5" /> by{" "}
					<Link
						href="https://zaid.qzz.io"
						className="underline"
						target="_blank"
						rel="noreferrer"
					>
						Zaid
					</Link>
				</div>
			</div>
		</footer>
	);
}

export default Footer;
