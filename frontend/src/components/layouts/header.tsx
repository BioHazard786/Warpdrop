import { ThemeSwitcher } from "@/components/ui/theme-switcher";

function Header() {
	return (
		<header className="fixed top-0 z-100 flex h-[53px] w-full items-center justify-end bg-background/95 px-4 backdrop-blur supports-backdrop-filter:bg-background/60">
			<ThemeSwitcher />
		</header>
	);
}

export default Header;
