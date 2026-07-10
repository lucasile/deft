<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	let { class: className, children, ...props }: HTMLAttributes<HTMLDivElement> & { children?: import('svelte').Snippet } =
		$props();
</script>

<div class={cn('border-b border-zinc-800 px-4 py-3', className)} {...props}>
	{@render children?.()}
</div>
