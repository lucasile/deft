<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Box, Clock3, Container, LogOut, ServerIcon } from '@lucide/svelte';
	import { auth, panel, type PanelEventPayload, type Server } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { backOrGoto } from '$lib/navigation';

	let server = $state<Server | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const serverID = $derived(page.params.serverID);

	onMount(() => {
		void loadServer();

		const events = panel.events();
		events.addEventListener('containers.changed', (event) => {
			const payload = parseEventPayload(event);
			if (!server || !payload.node_id || payload.node_id === server.node_id) {
				void loadServer({ quiet: true });
			}
		});
		events.addEventListener('command.updated', () => {
			void loadServer({ quiet: true });
		});
		return () => events.close();
	});

	const loadServer = async (options: { quiet?: boolean } = {}) => {
		if (!serverID) return;
		if (!options.quiet) {
			loading = true;
			error = null;
		}
		try {
			server = await panel.server(serverID);
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			if (!options.quiet) {
				loading = false;
			}
		}
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	type BadgeVariant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

	const statusVariant = (status = ''): BadgeVariant => {
		if (status === 'running') return 'success';
		if (status.endsWith('_requested')) return 'warning';
		if (status === 'failed' || status === 'missing') return 'destructive';
		return 'default';
	};

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
	};

	const formatDesiredConfig = (value?: string) => {
		if (!value) return '{}';
		try {
			return JSON.stringify(JSON.parse(value), null, 2);
		} catch {
			return value;
		}
	};

	const desiredConfig = $derived(formatDesiredConfig(server?.desired_config_json));

	const openNode = () => {
		if (!server) return;
		goto(`/nodes/${server.node_id}`);
	};

	const openContainer = () => {
		if (!server?.container_id) return;
		goto(`/nodes/${server.node_id}/containers/${server.container_id}`);
	};

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};

	const parseEventPayload = (event: Event): PanelEventPayload => {
		if (!(event instanceof MessageEvent) || typeof event.data !== 'string') return {};

		try {
			return JSON.parse(event.data) as PanelEventPayload;
		} catch {
			return {};
		}
	};
</script>

<svelte:head>
	<title>{server?.name || serverID} - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => backOrGoto('/')}>
					<ArrowLeft size={16} />
					Back
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">{server?.name || serverID}</h1>
				<p class="mt-1 truncate text-sm text-zinc-400">{server?.image || 'Server'}</p>
			</div>
			<div class="flex gap-2">
				<Button type="button" variant="outline" onclick={() => goto('/commands')}>
					<Clock3 size={16} />
					History
				</Button>
				<Button type="button" variant="outline" onclick={logout}>
					Log out
					<LogOut size={16} />
				</Button>
			</div>
		</div>
	</header>

	<div class="mx-auto grid max-w-6xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[0.85fr_1.15fr] lg:px-8">
		<section class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle>Overview</CardTitle>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if error}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{error}
						</div>
					{/if}

					{#if loading}
						<p class="text-sm text-zinc-400">Loading server...</p>
					{:else if !server}
						<p class="text-sm text-zinc-400">Server not found.</p>
					{:else}
						<div class="flex items-center justify-between gap-3">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-white">{server.name}</p>
								<p class="mt-1 truncate font-mono text-xs text-zinc-500">{server.id}</p>
							</div>
							<Badge variant={statusVariant(server.status)}>{server.status || 'unknown'}</Badge>
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
									Container
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

						<div class="grid gap-2 text-xs text-zinc-500">
							<p>Created {formatTime(server.created_at)}</p>
							<p>Updated {formatTime(server.updated_at)}</p>
						</div>
					{/if}
				</CardContent>
			</Card>
		</section>

		<section>
			<Card>
				<CardHeader>
					<CardTitle>Desired Config</CardTitle>
				</CardHeader>
				<CardContent>
					<pre class="max-h-[36rem] overflow-auto rounded-md border border-zinc-800 bg-zinc-950 p-3 text-xs text-zinc-300">{desiredConfig}</pre>
				</CardContent>
			</Card>
		</section>
	</div>
</main>
