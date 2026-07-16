<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { ArrowLeft, Clock3, LogOut, ServerIcon } from '@lucide/svelte';
	import { auth, panel, type Node } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { backOrGoto } from '$lib/navigation';

	let nodes = $state<Node[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const onlineNodes = $derived(
		nodes
			.filter((node) => node.connected)
			.sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id)),
	);

	onMount(() => {
		void loadNodes();
	});

	const loadNodes = async () => {
		loading = true;
		error = null;
		try {
			nodes = await panel.nodes();
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			loading = false;
		}
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};
</script>

<svelte:head>
	<title>Create Server - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-5xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => backOrGoto('/')}>
					<ArrowLeft size={16} />
					Back
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">Create server</h1>
				<p class="mt-1 truncate text-sm text-zinc-400">Choose where this server will run.</p>
			</div>
			<div class="flex gap-2">
				<Button type="button" variant="outline" onclick={() => goto('/commands')}>
					<Clock3 size={16} />
					History
				</Button>
				<Button type="button" variant="outline" onclick={logout}>
					<LogOut size={16} />
					Log out
				</Button>
			</div>
		</div>
	</header>

	<div class="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
		<Card>
			<CardHeader>
				<CardTitle>Online Nodes</CardTitle>
			</CardHeader>
			{#if error}
				<CardContent class="border-t border-zinc-800">
					<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
						{error}
					</div>
				</CardContent>
			{/if}

			{#if loading}
				<CardContent class="py-8 text-sm text-zinc-400">Loading nodes...</CardContent>
			{:else if onlineNodes.length === 0}
				<CardContent class="py-8 text-sm text-zinc-400">No online nodes are available.</CardContent>
			{:else}
				<div class="divide-y divide-zinc-800">
					{#each onlineNodes as node (node.id)}
						<button
							type="button"
							class="grid w-full gap-3 px-4 py-4 text-left hover:bg-zinc-800/70 sm:grid-cols-[1fr_auto]"
							onclick={() => goto(`/nodes/${node.id}/containers/new`)}
						>
							<div class="min-w-0">
								<div class="flex items-center gap-2">
									<ServerIcon size={16} class="text-zinc-400" />
									<span class="truncate font-medium text-white">{node.name || node.id}</span>
									<Badge variant="success">online</Badge>
								</div>
								<p class="mt-1 truncate font-mono text-xs text-zinc-500">{node.id}</p>
							</div>
							<p class="text-sm text-zinc-400">Use this node</p>
						</button>
					{/each}
				</div>
			{/if}
		</Card>
	</div>
</main>
