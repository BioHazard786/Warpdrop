import Logo from "@/components/logo";

function Hero() {
	return (
		<header className="text-center">
			<Logo />
			<p className="text-lg text-muted-foreground mt-2">
				Peer-to-Peer, encrypted file sharing.
			</p>
			<div className="mt-4 flex items-center justify-center gap-2">
				<div className="h-px w-12 bg-linear-to-r from-transparent to-cyan-800" />
				<p className="font-mono text-xs uppercase tracking-[0.2em] text-cyan-600">
					No limits. No Servers. Just Speed.
				</p>
				<div className="h-px w-12 bg-linear-to-l from-transparent to-cyan-800" />
			</div>
		</header>
	);
}

export default Hero;
