<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Check, ShieldCheck } from '@lucide/svelte';
	import { panel, type JoinRequestReview } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';

	let review = $state<JoinRequestReview | null>(null);
	let approvedNodeID = $state('');
	let loading = $state(true);
	let approving = $state(false);
	let error = $state<string | null>(null);

	const requestID = $derived(page.params.requestID ?? '');

	onMount(() => {
		void loadReview();
	});

	const loadReview = async () => {
		loading = true;
		error = null;
		try {
			review = await panel.joinRequest(requestID);
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			loading = false;
		}
	};

	const approve = async () => {
		approving = true;
		error = null;
		try {
			const result = await panel.approveJoinRequest(requestID);
			approvedNodeID = result.node_id;
			await loadReview();
			window.setTimeout(() => {
				goto('/');
			}, 3000);
		} catch (err) {
			error = cleanError(err);
		} finally {
			approving = false;
		}
	};

	const formatTime = (seconds: number) => new Date(seconds * 1000).toLocaleString();
	const cleanError = (err: unknown) => (err instanceof Error ? err.message.trim() : 'Request failed');
</script>

<main class="min-h-screen bg-zinc-950 px-4 py-8 text-zinc-100">
	<div class="mx-auto max-w-xl">
		<Card>
			<CardHeader>
				<CardTitle>Approve Node Join</CardTitle>
			</CardHeader>
			<CardContent class="space-y-5">
				{#if loading}
					<p class="text-sm text-zinc-400">Loading request...</p>
				{:else if review}
					<div class="flex items-center gap-3">
						<ShieldCheck class="text-emerald-400" size={24} />
						<div>
							<p class="font-medium text-white">{review.node_name || 'Unnamed node'}</p>
							<p class="text-sm text-zinc-400">Request {review.id}</p>
						</div>
					</div>

					<div class="rounded-md border border-zinc-800 bg-zinc-900 px-4 py-3">
						<p class="text-sm text-zinc-400">Verification code</p>
						<p class="mt-1 text-2xl font-semibold tracking-normal text-white">{review.verification_code}</p>
					</div>

					<div class="flex items-center justify-between text-sm text-zinc-400">
						<span>Expires {formatTime(review.expires_at)}</span>
						<Badge variant={review.status === 'pending' ? 'warning' : review.status === 'approved' ? 'success' : 'destructive'}>
							{review.status}
						</Badge>
					</div>

					{#if approvedNodeID}
						<div class="rounded-md border border-emerald-900/70 bg-emerald-950/50 px-3 py-2 text-sm text-emerald-200">
							Approved node {approvedNodeID}. Returning to dashboard...
						</div>
					{/if}

					<Button type="button" class="w-full" disabled={approving || review.status !== 'pending'} onclick={approve}>
						<Check size={16} />
						{approving ? 'Approving...' : 'Approve'}
					</Button>
				{/if}

				{#if error}
					<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
						{error}
					</div>
				{/if}
			</CardContent>
		</Card>
	</div>
</main>
