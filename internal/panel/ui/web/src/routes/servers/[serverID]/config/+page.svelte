<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Box, Clock3, Container, LogOut, ServerIcon } from '@lucide/svelte';
	import { auth, panel, type Server } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { backOrGoto } from '$lib/navigation';

	let server = $state<Server | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const serverID = $derived(page.params.serverID);
	const desiredConfig = $derived(formatDesiredConfig(server?.desired_config_json));

	onMount(() => {
		void loadServer();
	});

	const loadServer = async () => {
		if (!serverID) return;
		loading = true;
		error = null;
		try {
			server = await panel.server(serverID);
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

	const openNode = () => {
		if (!server) return;
		goto(`/nodes/${server.node_id}`);
	};

	const openContainer = () => {
		if (!server?.container_id) return;
		goto(`/nodes/${server.node_id}/containers/${server.container_id}?from=${encodeURIComponent(`/servers/${serverID}/config`)}`);
	};

	function formatDesiredConfig(value?: string) {
		if (!value) return '{}';
		try {
			return JSON.stringify(JSON.parse(value), null, 2);
		} catch {
			return value;
		}
	}

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};
</script>

<svelte:head>
	<title>{server?.name || serverID} Config - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button
					type="button"
					variant="ghost"
					class="mb-2 px-0 text-zinc-400 hover:text-white"
					onclick={() => backOrGoto(`/servers/${serverID}`)}
				>
					<ArrowLeft size={16} />
					Back
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">{server?.name || serverID} config</h1>
				<p class="mt-1 truncate text-sm text-zinc-400">Docker-backed settings and advanced links</p>
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

	<div class="mx-auto grid max-w-6xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[0.85fr_1.15fr] lg:px-8">
		<section>
			<Card>
				<CardHeader>
					<CardTitle>Config Summary</CardTitle>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if error}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{error}
						</div>
					{/if}

					{#if loading}
						<p class="text-sm text-zinc-400">Loading config...</p>
					{:else if !server}
						<p class="text-sm text-zinc-400">Server not found.</p>
					{:else}
						<div class="flex items-center justify-between gap-3">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-white">{server.name}</p>
								<p class="mt-1 truncate font-mono text-xs text-zinc-500">{server.id}</p>
							</div>
							<Badge>{server.status || 'unknown'}</Badge>
						</div>

						<div class="grid gap-3 text-sm">
							<div class="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2">
								<div class="flex items-center gap-2 text-zinc-400">
									<ServerIcon size={15} />
									Node
								</div>
								<button type="button" class="mt-1 truncate text-left font-mono text-zinc-100 hover:text-emerald-300" onclick={openNode}>
									{server.node_id}
								</button>
							</div>

							<div class="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2">
								<div class="flex items-center gap-2 text-zinc-400">
									<Box size={15} />
									Image
								</div>
								<p class="mt-1 truncate font-mono text-zinc-100">{server.image}</p>
							</div>

							<div class="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2">
								<div class="flex items-center gap-2 text-zinc-400">
									<Container size={15} />
									Advanced container
								</div>
								{#if server.container_id}
									<button
										type="button"
										class="mt-1 break-all text-left font-mono text-zinc-100 hover:text-emerald-300"
										onclick={openContainer}
									>
										{server.container_id}
									</button>
								{:else}
									<p class="mt-1 text-zinc-500">No linked container yet.</p>
								{/if}
							</div>
						</div>
					{/if}
				</CardContent>
			</Card>
		</section>

		<section>
			<Card>
				<CardHeader>
					<CardTitle>Desired Docker Config</CardTitle>
				</CardHeader>
				<CardContent>
					<pre class="max-h-[36rem] overflow-auto rounded-md border border-zinc-800 bg-zinc-950 p-3 text-xs text-zinc-300">{desiredConfig}</pre>
				</CardContent>
			</Card>
		</section>
	</div>
</main>
