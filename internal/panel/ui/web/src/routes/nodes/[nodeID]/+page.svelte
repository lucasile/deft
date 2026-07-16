<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Clock3, LogOut, Plus, Power, RefreshCw, Trash2 } from '@lucide/svelte';
	import { auth, panel, type Container, type Node, type PanelEventPayload } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';

	let nodes = $state<Node[]>([]);
	let containers = $state<Container[]>([]);
	let selectedContainerID = $state('');
	let loading = $state(true);
	let containersLoading = $state(false);
	let busy = $state(false);
	let error = $state<string | null>(null);
	let confirmAction = $state<'stop-agent' | 'remove-agent' | null>(null);
	let pendingCreates = $state<Record<string, Container>>({});

	const nodeID = $derived(page.params.nodeID);
	const selectedNode = $derived(nodes.find((node) => node.id === nodeID));
	const visibleContainers = $derived([...Object.values(pendingCreates), ...containers]);

	onMount(() => {
		loadPendingCreates();
		void loadNodes();
		void loadContainers({ quiet: true });

		const events = panel.events();
		events.addEventListener('nodes.changed', () => {
			void loadNodes({ quiet: true });
		});
		events.addEventListener('command.updated', (event) => {
			const payload = parseEventPayload(event);
			if (payload.command_id) void handleCommandUpdated(payload.command_id);
		});
		events.addEventListener('containers.changed', (event) => {
			const payload = parseEventPayload(event);
			if (!payload.node_id || payload.node_id === nodeID) {
				void loadContainers({ quiet: true });
			}
		});
		return () => events.close();
	});

	const loadNodes = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) {
			loading = true;
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
			loading = false;
		}
	};

	const loadContainers = async (options: { quiet?: boolean } = {}) => {
		if (!nodeID) return;
		if (!options.quiet) {
			containersLoading = true;
			error = null;
		}
		try {
			containers = await panel.containers(nodeID);
			if (containers.length === 0) {
				selectedContainerID = '';
			} else if (!selectedContainerID || !containers.some((container) => container.id === selectedContainerID)) {
				selectedContainerID = containers[0].id;
			}
		} catch (err) {
			error = cleanError(err);
		} finally {
			if (!options.quiet) {
				containersLoading = false;
			}
		}
	};

	const runCommand = async (operation: () => Promise<void>) => {
		busy = true;
		error = null;
		try {
			await operation();
			await loadNodes({ quiet: true });
			await loadContainers({ quiet: true });
		} catch (err) {
			error = cleanError(err);
		} finally {
			busy = false;
		}
	};

	const handleCommandUpdated = async (commandID: string) => {
		if (pendingCreates[commandID]) {
			try {
				const command = await panel.command(commandID);
				if (command.status !== 'pending') {
					const { [commandID]: pending, ...remainingCreates } = pendingCreates;
					pendingCreates = remainingCreates;
					if (command.status === 'failed') {
						error = command.message || 'Failed to create container.';
					}
				}
			} catch {
				const { [commandID]: pending, ...remainingCreates } = pendingCreates;
				pendingCreates = remainingCreates;
			}
		}
		await loadContainers({ quiet: true });
	};

	const loadPendingCreates = () => {
		if (!nodeID) return;
		const storageKey = pendingCreateStorageKey(nodeID);
		const rawValue = sessionStorage.getItem(storageKey);
		if (!rawValue) return;
		sessionStorage.removeItem(storageKey);

		try {
			const pending = JSON.parse(rawValue) as Container[];
			pendingCreates = Object.fromEntries(pending.map((container) => [container.id, container]));
		} catch {
			pendingCreates = {};
		}
	};

	const removeNode = async () => {
		if (!selectedNode || selectedNode.connected) return;

		error = null;
		try {
			await panel.removeNode(selectedNode.id);
			goto('/');
		} catch (err) {
			error = cleanError(err);
		}
	};

	const stopAgent = async () => {
		if (!selectedNode?.connected) return;

		await runCommand(async () => {
			await panel.stopNode(selectedNode.id);
		});
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
	};

	type BadgeVariant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

	const containerStatusVariant = (status = ''): BadgeVariant => {
		if (status === 'running') return 'success';
		if (status.endsWith('_requested')) return 'warning';
		return 'default';
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

	const pendingCreateStorageKey = (id: string) => `deft.pending-creates.${id}`;
</script>

<svelte:head>
	<title>{selectedNode?.name || nodeID} - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => goto('/')}>
					<ArrowLeft size={16} />
					Nodes
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">{selectedNode?.name || nodeID}</h1>
				<p class="mt-1 truncate text-sm text-zinc-400">{nodeID}</p>
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

	<div class="mx-auto grid max-w-6xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[0.8fr_1.2fr] lg:px-8">
		<section class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle>Agent</CardTitle>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if error}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{error}
						</div>
					{/if}

					{#if loading}
						<p class="text-sm text-zinc-400">Loading agent...</p>
					{:else if !selectedNode}
						<p class="text-sm text-zinc-400">Agent not found.</p>
					{:else}
						<div class="space-y-3">
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium text-zinc-200">{selectedNode.name || selectedNode.id}</p>
									<p class="mt-1 truncate font-mono text-xs text-zinc-500">{selectedNode.id}</p>
								</div>
								<Badge variant={selectedNode.connected ? 'success' : 'default'}>
									{selectedNode.connected ? 'online' : 'offline'}
								</Badge>
							</div>
							<p class="text-sm text-zinc-400">Last seen {formatTime(selectedNode.last_seen)}</p>
							<div class="rounded-md border border-amber-900/60 bg-amber-950/40 px-3 py-2 text-sm text-amber-100">
								Remote stop shuts down this agent process. You cannot start it from the panel afterward; start it again on the agent machine.
							</div>
							<Button
								type="button"
								variant="destructive"
								class="w-full"
								disabled={busy || !selectedNode.connected}
								onclick={() => (confirmAction = 'stop-agent')}
							>
								<Power size={15} />
								Stop agent remotely
							</Button>
							<Button
								type="button"
								variant="destructive"
								class="w-full"
								disabled={busy || selectedNode.connected}
								onclick={() => (confirmAction = 'remove-agent')}
							>
								<Trash2 size={15} />
								Remove agent
							</Button>
							{#if selectedNode.connected}
								<p class="text-xs text-zinc-500">Stop the agent before removing it from the panel.</p>
							{/if}
						</div>
					{/if}
				</CardContent>
			</Card>
		</section>

		<section class="space-y-6">
			<Card>
				<CardHeader class="flex flex-row items-center justify-between">
					<div>
						<CardTitle>Containers</CardTitle>
						<p class="text-sm text-zinc-400">{visibleContainers.length} known on this agent</p>
					</div>
					<div class="flex gap-2">
						<Button type="button" size="sm" disabled={!selectedNode?.connected} onclick={() => goto(`/nodes/${nodeID}/containers/new`)}>
							<Plus size={14} />
							Create
						</Button>
						<Button type="button" variant="outline" size="sm" disabled={containersLoading} onclick={() => void loadContainers()}>
							<RefreshCw size={14} />
							Refresh
						</Button>
					</div>
				</CardHeader>

				{#if containersLoading}
					<CardContent class="py-8 text-sm text-zinc-400">Loading containers...</CardContent>
				{:else if visibleContainers.length === 0}
					<CardContent class="py-8 text-sm text-zinc-400">No containers known for this agent.</CardContent>
				{:else}
					<div class="divide-y divide-zinc-800">
						{#each visibleContainers as container (container.id)}
							<button
								type="button"
								class={`grid w-full gap-3 px-4 py-4 text-left hover:bg-zinc-800/70 sm:grid-cols-[1fr_auto] ${container.id === selectedContainerID ? 'bg-zinc-800' : ''}`}
								disabled={container.status === 'create_requested'}
								onclick={() => goto(`/nodes/${nodeID}/containers/${container.id}`)}
							>
								<div class="min-w-0">
									<p class="truncate text-sm font-medium text-white">{container.name || container.id}</p>
									{#if container.image}
										<p class="mt-1 truncate text-xs text-zinc-400">{container.image}</p>
									{/if}
									<p class="mt-1 truncate font-mono text-xs text-zinc-500">{container.id}</p>
								</div>
								<Badge class="h-fit" variant={containerStatusVariant(container.status)}>
									{container.status || 'unknown'}
								</Badge>
							</button>
						{/each}
					</div>
				{/if}
			</Card>
		</section>
	</div>
	<ConfirmDialog
		bind:open={() => confirmAction === 'stop-agent', (value) => {
			if (!value) confirmAction = null;
		}}
		title="Stop agent remotely?"
		description={`The panel cannot start "${selectedNode?.name || selectedNode?.id || 'this agent'}" again. You must log into that machine and start the agent there.`}
		confirmLabel="Stop agent"
		onconfirm={stopAgent}
	/>
	<ConfirmDialog
		bind:open={() => confirmAction === 'remove-agent', (value) => {
			if (!value) confirmAction = null;
		}}
		title="Remove offline agent?"
		description={`Remove "${selectedNode?.name || selectedNode?.id || 'this agent'}" from this panel? This deletes its panel containers and command history.`}
		confirmLabel="Remove agent"
		onconfirm={removeNode}
	/>
</main>
