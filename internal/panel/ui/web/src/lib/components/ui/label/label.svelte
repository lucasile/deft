<script lang="ts">
	import type { HTMLLabelAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils';

	let { class: className, children, ...props }: HTMLLabelAttributes & { children?: import('svelte').Snippet } =
		$props();
</script>

<label class={cn('text-sm font-medium text-zinc-300', className)} {...props}>
	{@render children?.()}
</label>
