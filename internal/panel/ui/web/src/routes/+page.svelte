<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Check, Clock3, Copy, KeyRound, LogOut, Plus, RefreshCw } from '@lucide/svelte';
	import { auth, panel, type JoinTokenInfo, type Node, type PanelEventPayload, type Server } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';

	let nodes = $state<Node[]>([]);
	let servers = $state<Server[]>([]);
	let nodesLoading = $state(true);
	let serversLoading = $state(true);
	let joinTokenLoading = $state(false);
	let joinToken = $state('');
	let joinTokenExpiresAt = $state(0);
	let joinTokens = $state<JoinTokenInfo[]>([]);
	let joinTokenCopied = $state(false);
	let error = $state<string | null>(null);
	let tokenToRevoke = $state<string | null>(null);

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const sortedServers = $derived([...servers].sort((a, b) => b.created_at - a.created_at || a.name.localeCompare(b.name)));
	const onlineNodes = $derived(
		nodes
			.filter((node) => node.connected)
			.sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id)),
	);
	const offlineNodes = $derived(
		nodes
			.filter((node) => !node.connected)
			.sort((a, b) => {
				const aJoinedAt = a.created_at || a.last_seen;
				const bJoinedAt = b.created_at || b.last_seen;
				if (aJoinedAt !== bJoinedAt) return bJoinedAt - aJoinedAt;
				return (a.name || a.id).localeCompare(b.name || b.id);
			}),
	);
	const nodeSections = $derived(
		[
			{ label: 'Online', nodes: onlineNodes },
			{ label: 'Offline', nodes: offlineNodes },
		].filter((section) => section.nodes.length > 0),
	);
	const duplicateNodeNames = $derived(
		nodes.reduce<Record<string, number>>((counts, node) => {
			const name = node.name || node.id;
			counts[name] = (counts[name] ?? 0) + 1;
			return counts;
		}, {}),
	);
	const activeJoinTokenCount = $derived(joinTokens.filter((token) => token.status === 'active').length);

	onMount(() => {
		void loadNodes();
		void loadServers();
		void loadJoinTokens();

		const events = panel.events();
		events.addEventListener('nodes.changed', () => {
			void loadNodes({ quiet: true });
		});
		events.addEventListener('containers.changed', () => {
			void loadServers({ quiet: true });
		});
		events.addEventListener('command.updated', (event) => {
			const payload = parseEventPayload(event);
			if (payload.command_id) void loadServers({ quiet: true });
		});
		return () => events.close();
	});

	const loadNodes = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) {
			nodesLoading = true;
			error = null;
		}
		try {
			nodes = await panel.nodes();
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			nodesLoading = false;
		}
	};

	const loadServers = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) {
			serversLoading = true;
			error = null;
		}
		try {
			servers = await panel.servers();
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			serversLoading = false;
		}
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	const createJoinToken = async () => {
		joinTokenLoading = true;
		error = null;
		try {
			const response = await panel.createJoinToken();
			joinToken = response.token;
			joinTokenExpiresAt = response.expires_at;
			joinTokenCopied = false;
			await loadJoinTokens();
		} catch (err) {
			error = cleanError(err);
		} finally {
			joinTokenLoading = false;
		}
	};

	const loadJoinTokens = async () => {
		try {
			joinTokens = await panel.joinTokens();
		} catch (err) {
			error = cleanError(err);
		}
	};

	const revokeJoinToken = async () => {
		if (!tokenToRevoke) return;

		error = null;
		try {
			await panel.revokeJoinToken(tokenToRevoke);
			await loadJoinTokens();
		} catch (err) {
			error = cleanError(err);
		} finally {
			tokenToRevoke = null;
		}
	};

	const copyJoinToken = async () => {
		if (!joinToken) return;

		try {
			await navigator.clipboard.writeText(joinToken);
			joinTokenCopied = true;
			window.setTimeout(() => {
				joinTokenCopied = false;
			}, 1500);
		} catch (err) {
			error = cleanError(err);
		}
	};

	const openNode = (nodeID: string) => {
		goto(`/nodes/${nodeID}`);
	};

	const openServer = (serverID: string) => {
		goto(`/servers/${serverID}`);
	};

	const createServer = () => {
		if (onlineNodes.length === 1) {
			goto(`/nodes/${onlineNodes[0].id}/containers/new`);
			return;
		}
		goto('/servers/new');
	};

	const isDuplicateNodeName = (node: Node) => {
		return (duplicateNodeNames[node.name || node.id] ?? 0) > 1;
	};

	const nodeSortTime = (node: Node) => {
		return node.connected ? node.last_seen : node.created_at || node.last_seen;
	};

	const nodeSortLabel = (node: Node) => {
		return node.connected ? 'Last seen' : 'Joined';
	};

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
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

	type BadgeVariant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

	const serverStatusVariant = (status = ''): BadgeVariant => {
		if (status === 'running') return 'success';
		if (status.endsWith('_requested')) return 'warning';
		if (status === 'failed' || status === 'missing') return 'destructive';
		return 'default';
	};

</script>

<svelte:head>
	<title>Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-5xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div>
				<h1 class="text-xl font-semibold tracking-normal text-white">Deft</h1>
				<p class="mt-1 text-sm text-zinc-400">Server and node control</p>
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

	<div class="mx-auto max-w-5xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
		<Card>
			<CardHeader class="flex flex-row items-center justify-between">
				<div>
					<CardTitle>Servers</CardTitle>
					<p class="text-sm text-zinc-400">{servers.length} known</p>
				</div>
				<div class="flex gap-2">
					<Button type="button" size="sm" disabled={onlineNodes.length === 0} onclick={createServer}>
						<Plus size={15} />
						Create server
					</Button>
					<Button type="button" variant="outline" size="sm" disabled={serversLoading} onclick={() => void loadServers()}>
						<RefreshCw size={15} />
						Refresh
					</Button>
				</div>
			</CardHeader>

			{#if serversLoading && servers.length === 0}
				<CardContent class="py-8 text-sm text-zinc-400">Loading servers...</CardContent>
			{:else if sortedServers.length === 0}
				<CardContent class="space-y-3 py-8">
					<p class="text-sm text-zinc-400">No servers have been created yet.</p>
					<Button type="button" disabled={onlineNodes.length === 0} onclick={createServer}>
						<Plus size={16} />
						Create server
					</Button>
					{#if onlineNodes.length === 0}
						<p class="text-xs text-zinc-500">Connect an agent before creating a server.</p>
					{/if}
				</CardContent>
			{:else}
				<div class="divide-y divide-zinc-800">
					{#each sortedServers as server (server.id)}
						<button
							type="button"
							class="grid w-full gap-3 px-4 py-4 text-left hover:bg-zinc-800/70 sm:grid-cols-[1fr_auto]"
							onclick={() => openServer(server.id)}
						>
							<div class="min-w-0">
								<div class="flex items-center gap-2">
									<span class="truncate font-medium text-white">{server.name}</span>
									<Badge variant={serverStatusVariant(server.status)}>
										{server.status || 'unknown'}
									</Badge>
								</div>
								<p class="mt-1 truncate text-sm text-zinc-400">{server.image}</p>
								<p class="mt-1 truncate font-mono text-xs text-zinc-500">{server.id}</p>
							</div>
							<div class="min-w-0 text-sm text-zinc-400">
								<p class="truncate">Node {server.node_id}</p>
								{#if server.container_id}
									<p class="mt-1 truncate font-mono text-xs text-zinc-500">{server.container_id}</p>
								{/if}
							</div>
						</button>
					{/each}
				</div>
			{/if}
		</Card>

		<Card>
			<CardHeader class="flex flex-row items-center justify-between">
				<div>
					<CardTitle>Nodes</CardTitle>
					<p class="text-sm text-zinc-400">{connectedNodes.length} connected / {nodes.length} known</p>
				</div>
				<div class="flex gap-2">
					<Button
						type="button"
						variant="outline"
						size="sm"
						disabled={joinTokenLoading || activeJoinTokenCount >= 5}
						onclick={createJoinToken}
					>
						<KeyRound size={15} />
						{activeJoinTokenCount >= 5 ? 'Token limit reached' : 'Get join token'}
					</Button>
					<Button type="button" variant="outline" size="sm" disabled={nodesLoading} onclick={() => void loadNodes()}>
						<RefreshCw size={15} />
						Refresh
					</Button>
				</div>
			</CardHeader>

			{#if error}
				<CardContent class="border-t border-zinc-800">
					<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
						{error}
					</div>
				</CardContent>
			{/if}

			{#if joinToken}
				<CardContent class="border-t border-zinc-800">
					<div class="grid gap-3 rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 sm:grid-cols-[1fr_auto]">
						<div class="min-w-0">
							<p class="text-sm text-zinc-400">Join token expires {formatTime(joinTokenExpiresAt)}</p>
							<p class="mt-1 break-all font-mono text-sm text-emerald-300">{joinToken}</p>
						</div>
						<Button type="button" variant="outline" size="sm" onclick={copyJoinToken}>
							{#if joinTokenCopied}
								<Check size={15} />
								Copied
							{:else}
								<Copy size={15} />
								Copy
							{/if}
						</Button>
					</div>
				</CardContent>
			{/if}

			{#if joinTokens.length > 0}
				<div class="border-t border-zinc-800 px-4 py-3">
					<p class="text-sm font-medium text-zinc-200">Join token controls</p>
					<p class="mb-2 mt-1 text-xs text-zinc-500">
						Token secrets are shown once. Revoke active tokens that were copied to the wrong place or no longer needed.
					</p>
					<div class="space-y-2">
						{#each joinTokens as token (token.id)}
							<div class="grid gap-2 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 sm:grid-cols-[1fr_auto]">
								<div class="min-w-0">
									<div class="flex items-center gap-2">
										<Badge
											variant={token.status === 'active'
												? 'success'
												: token.status === 'expired'
													? 'default'
													: token.status === 'revoked'
														? 'destructive'
														: 'warning'}
										>
											{token.status}
										</Badge>
										<span class="truncate text-sm text-zinc-300">{token.node_name || token.id}</span>
									</div>
									<p class="mt-1 text-xs text-zinc-500">Expires {formatTime(token.expires_at)}</p>
								</div>
								<Button
									type="button"
									variant="destructive"
									size="sm"
									disabled={token.status !== 'active'}
									onclick={() => (tokenToRevoke = token.id)}
								>
									Revoke
								</Button>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			{#if nodes.length === 0 && nodesLoading}
				<CardContent class="py-8 text-sm text-zinc-400">Loading nodes...</CardContent>
			{:else if nodes.length === 0}
				<CardContent class="py-8 text-sm text-zinc-400">No nodes have connected yet.</CardContent>
			{:else}
				<div class="divide-y divide-zinc-800">
					{#each nodeSections as section (section.label)}
						<div>
							<div class="bg-zinc-950 px-4 py-2 text-xs font-medium uppercase tracking-normal text-zinc-500">
								{section.label}
							</div>
							<div class="divide-y divide-zinc-800">
								{#each section.nodes as node (node.id)}
									<button
										type="button"
										class="grid w-full gap-3 px-4 py-4 text-left hover:bg-zinc-800/70 sm:grid-cols-[1fr_auto]"
										onclick={() => openNode(node.id)}
									>
										<div class="min-w-0">
											<div class="flex items-center gap-2">
												<span class="truncate font-medium text-white">{node.name || node.id}</span>
												<Badge variant={node.connected ? 'success' : 'default'}>
													{node.connected ? 'connected' : 'offline'}
												</Badge>
											</div>
											<p class="mt-1 truncate text-sm text-zinc-400">
												{node.id}
												{#if isDuplicateNodeName(node)}
													<span class="text-zinc-500"> duplicate name</span>
												{/if}
											</p>
										</div>
										<div class="text-sm text-zinc-400">{nodeSortLabel(node)} {formatTime(nodeSortTime(node))}</div>
									</button>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</Card>

	</div>
	<ConfirmDialog
		bind:open={() => Boolean(tokenToRevoke), (value) => {
			if (!value) tokenToRevoke = null;
		}}
		title="Revoke join token?"
		description="Agents will no longer be able to use this join token."
		confirmLabel="Revoke"
		onconfirm={revokeJoinToken}
	/>
</main>
