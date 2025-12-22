"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";
import { Button } from "./button";

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
		<div className={cn("rounded-full bg-ring/30 relative flex", className)}>
			{themes.map(({ key, icon: Icon, label }) => {
				const isActive = theme === key;
				return (
					<button
						aria-label={label}
						className="relative size-7 md:size-8 rounded-full cursor-pointer flex items-center justify-center"
						key={key}
						onClick={() => setTheme(key as "light" | "dark" | "system")}
						type="button"
					>
						{isActive && (
							<div className="absolute inset-0 rounded-full bg-muted-foreground/30" />
						)}
						<Icon
							className={cn(
								"relative z-10 size-3.5 md:size-4",
								isActive
									? "text-secondary-foreground"
									: "text-muted-foreground",
							)}
						/>
					</button>
				);
			})}
		</div>
	);
};
