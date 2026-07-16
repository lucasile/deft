<script lang="ts">
	import { Button } from '$lib/components/ui/button';

	let {
		open = $bindable(false),
		title,
		description,
		confirmLabel = 'Confirm',
		cancelLabel = 'Cancel',
		destructive = true,
		onconfirm,
	}: {
		open: boolean;
		title: string;
		description: string;
		confirmLabel?: string;
		cancelLabel?: string;
		destructive?: boolean;
		onconfirm: () => void | Promise<void>;
	} = $props();

	const close = () => {
		open = false;
	};

	const confirm = async () => {
		await onconfirm();
		close();
	};
</script>

{#if open}
	<div class="fixed inset-0 z-50 grid place-items-center px-4">
		<button class="absolute inset-0 bg-black/70" type="button" aria-label="Close dialog" onclick={close}></button>
		<div
			class="relative w-full max-w-md rounded-md border border-zinc-800 bg-zinc-950 p-5 text-zinc-100 shadow-xl"
			role="alertdialog"
			tabindex="-1"
			aria-modal="true"
			aria-labelledby="confirm-title"
			aria-describedby="confirm-description"
		>
			<h2 id="confirm-title" class="text-lg font-semibold text-white">{title}</h2>
			<p id="confirm-description" class="mt-2 text-sm leading-6 text-zinc-300">{description}</p>
			<div class="mt-5 flex justify-end gap-2">
				<Button type="button" variant="outline" onclick={close}>{cancelLabel}</Button>
				<Button type="button" variant={destructive ? 'destructive' : 'default'} onclick={confirm}>
					{confirmLabel}
				</Button>
			</div>
		</div>
	</div>
{/if}
