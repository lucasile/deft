<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	let { class: className, children, ...props }: HTMLAttributes<HTMLDivElement> & { children?: import('svelte').Snippet } =
		$props();
</script>

<div class={cn('p-4', className)} {...props}>
	{@render children?.()}
</div>
