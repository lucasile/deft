<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	let { class: className, children, ...props }: HTMLAttributes<HTMLHeadingElement> & {
		children?: import('svelte').Snippet;
	} = $props();
</script>

<h2 class={cn('text-base font-medium text-white', className)} {...props}>
	{@render children?.()}
</h2>
