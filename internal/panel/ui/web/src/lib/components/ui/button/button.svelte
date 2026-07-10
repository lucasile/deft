<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	type Variant = 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive';
	type Size = 'default' | 'sm' | 'icon';

	let {
		class: className,
		variant = 'default',
		size = 'default',
		children,
		...props
	}: HTMLButtonAttributes & {
		variant?: Variant;
		size?: Size;
		children?: import('svelte').Snippet;
	} = $props();

	const variantClass: Record<Variant, string> = {
		default: 'bg-emerald-600 text-white hover:bg-emerald-500',
		secondary: 'bg-zinc-800 text-zinc-100 hover:bg-zinc-700',
		outline: 'border border-zinc-700 bg-transparent text-zinc-100 hover:bg-zinc-800',
		ghost: 'text-zinc-200 hover:bg-zinc-800',
		destructive: 'border border-red-900/70 bg-transparent text-red-200 hover:bg-red-950',
	};

	const sizeClass: Record<Size, string> = {
		default: 'h-10 px-4 py-2',
		sm: 'h-9 px-3',
		icon: 'h-10 w-10',
	};
</script>

<button
	class={cn(
		'inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50',
		variantClass[variant],
		sizeClass[size],
		className,
	)}
	{...props}
>
	{@render children?.()}
</button>
