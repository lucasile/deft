<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	type Variant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

	let {
		class: className,
		variant = 'default',
		children,
		...props
	}: HTMLAttributes<HTMLSpanElement> & {
		variant?: Variant;
		children?: import('svelte').Snippet;
	} = $props();

	const variantClass: Record<Variant, string> = {
		default: 'bg-zinc-700 text-zinc-200',
		secondary: 'bg-zinc-800 text-zinc-300',
		success: 'bg-emerald-500/15 text-emerald-300',
		warning: 'bg-amber-500/15 text-amber-300',
		destructive: 'bg-red-500/15 text-red-300',
	};
</script>

<span class={cn('inline-flex rounded-full px-2 py-0.5 text-xs font-medium', variantClass[variant], className)} {...props}>
	{@render children?.()}
</span>
