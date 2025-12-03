"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

const themes = [
	{
		key: "system",
		icon: Monitor,
		label: "System theme",
	},
	{
		key: "light",
		icon: Sun,
		label: "Light theme",
	},
	{
		key: "dark",
		icon: Moon,
		label: "Dark theme",
	},
];

export const ThemeSwitcher = ({ className }: { className?: string }) => {
	const { theme, setTheme } = useTheme();

	return (
		<div
			className={cn(
				"relative isolate flex h-8 rounded-full bg-background p-1 ring-1 ring-border",
				className,
			)}
		>
			{themes.map(({ key, icon: Icon, label }) => {
				const isActive = theme === key;
				return (
					<button
						aria-label={label}
						className="relative h-6 w-6 rounded-full cursor-pointer"
						key={key}
						onClick={() => setTheme(key as "light" | "dark" | "system")}
						type="button"
					>
						{isActive && (
							<div className="absolute inset-0 rounded-full bg-secondary" />
						)}
						<Icon
							className={cn(
								"relative z-10 m-auto h-4 w-4",
								isActive ? "text-foreground" : "text-muted-foreground",
							)}
						/>
					</button>
				);
			})}
		</div>
	);
};
