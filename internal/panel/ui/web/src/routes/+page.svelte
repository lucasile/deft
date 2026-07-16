<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { Check, Copy, KeyRound, LogOut, Play, Plus, RefreshCw, Square, Trash2 } from '@lucide/svelte';
	import { defaults, superForm } from 'sveltekit-superforms';
	import { zod4 } from 'sveltekit-superforms/adapters';
	import {
		auth,
		panel,
		type Command,
		type Container,
		type JoinTokenInfo,
		type Node,
		type PanelEventPayload,
	} from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { createContainerSchema } from '$lib/schemas';

	let nodes = $state<Node[]>([]);
	let containers = $state<Container[]>([]);
	let selectedNodeID = $state('');
	let selectedContainerID = $state('');
	let loading = $state(true);
	let containersLoading = $state(false);
	let busy = $state(false);
	let createSubmitting = $state(false);
	let joinTokenLoading = $state(false);
	let joinToken = $state('');
	let joinTokenExpiresAt = $state(0);
	let joinTokens = $state<JoinTokenInfo[]>([]);
	let joinTokenCopied = $state(false);
	let error = $state<string | null>(null);
	let commands = $state<Command[]>([]);
	const trackedCommandIDs = new SvelteSet<string>();

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const selectedNode = $derived(nodes.find((node) => node.id === selectedNodeID));
	const selectedContainer = $derived(containers.find((container) => container.id === selectedContainerID));
	const activeJoinTokenCount = $derived(joinTokens.filter((token) => token.status === 'active').length);
	const createContainerForm = superForm(
		defaults({ name: 'hello-nginx', image: 'nginx:alpine' }, zod4(createContainerSchema)),
		{
			SPA: true,
			validators: zod4(createContainerSchema),
			async onUpdate({ form }) {
				if (!form.valid || createSubmitting || !selectedNodeID) return;

				createSubmitting = true;
				await runCommand(async () => {
					const response = await panel.createContainer(selectedNodeID, form.data.name, form.data.image);
					selectedContainerID = form.data.name;
					await trackCommand(response.command_id);
				});
				createSubmitting = false;
			},
		},
	);

	const {
		form: createForm,
		errors: createErrors,
		constraints: createConstraints,
		enhance: enhanceCreate,
	} = createContainerForm;

	onMount(() => {
		void loadNodes().then(() => loadContainers({ quiet: true }));
		void loadJoinTokens();

		const events = panel.events();
		events.addEventListener('nodes.changed', () => {
			void loadNodes({ quiet: true });
		});
		events.addEventListener('command.updated', (event) => {
			const payload = parseEventPayload(event);
			if (payload.command_id && trackedCommandIDs.has(payload.command_id)) {
				void refreshCommand(payload.command_id);
			}
			void loadContainers({ quiet: true });
		});
		events.addEventListener('containers.changed', (event) => {
			const payload = parseEventPayload(event);
			if (!payload.node_id || payload.node_id === selectedNodeID) {
				void loadContainers({ quiet: true });
			}
		});
		events.onerror = () => {
			if (events.readyState === EventSource.CLOSED) {
				error = 'Lost panel event stream';
			}
		};

		return () => events.close();
	});

	const loadNodes = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) {
			loading = true;
			error = null;
		}
		try {
			nodes = await panel.nodes();
			if (!selectedNodeID && nodes.length > 0) {
				selectedNodeID = nodes[0].id;
			}
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

	const loadContainers = async (options: { quiet?: boolean } = {}) => {
		if (!selectedNodeID) {
			containers = [];
			selectedContainerID = '';
			return;
		}
		if (!options.quiet) {
			containersLoading = true;
			error = null;
		}
		try {
			containers = await panel.containers(selectedNodeID);
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

	const selectNode = (nodeID: string) => {
		selectedNodeID = nodeID;
		selectedContainerID = '';
		void loadContainers();
	};

	const runContainerAction = async (action: 'start' | 'stop' | 'remove') => {
		if (!selectedNodeID || !selectedContainer) return;

		await runCommand(async () => {
			const response = await panel.containerAction(selectedNodeID, selectedContainer.id, action);
			await trackCommand(response.command_id);
		});
	};

	const runCommand = async (operation: () => Promise<void>) => {
		busy = true;
		error = null;
		try {
			await operation();
			await loadNodes();
			await loadContainers({ quiet: true });
		} catch (err) {
			error = cleanError(err);
		} finally {
			busy = false;
		}
	};

	const trackCommand = async (commandID: string) => {
		trackedCommandIDs.add(commandID);
		await refreshCommand(commandID);
	};

	const refreshCommand = async (commandID: string) => {
		const command = await panel.command(commandID);
		commands = [command, ...commands.filter((item) => item.id !== command.id)].slice(0, 8);
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

	const revokeJoinToken = async (tokenID: string) => {
		error = null;
		try {
			await panel.revokeJoinToken(tokenID);
			await loadJoinTokens();
		} catch (err) {
			error = cleanError(err);
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

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
	};

	type BadgeVariant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

	const containerStatusVariant = (status = ''): BadgeVariant => {
		if (status === 'started') return 'success';
		if (status.endsWith('_requested')) return 'warning';
		if (status === 'removed') return 'destructive';
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
</script>

<svelte:head>
	<title>Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div>
				<h1 class="text-xl font-semibold tracking-normal text-white">Deft</h1>
				<p class="mt-1 text-sm text-zinc-400">Node and container control</p>
			</div>
			<Button
				type="button"
				variant="outline"
				onclick={logout}
			>
				<LogOut size={16} />
				Log out
			</Button>
		</div>
	</header>

	<div class="mx-auto grid max-w-7xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[1.25fr_0.75fr] lg:px-8">
		<section class="space-y-6">
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
						<Button
							type="button"
							variant="outline"
							size="sm"
							disabled={loading}
							onclick={() => void loadNodes()}
						>
							<RefreshCw size={15} />
							Refresh
						</Button>
					</div>
				</CardHeader>

				{#if joinToken}
					<CardContent class="border-t border-zinc-800">
						<div class="grid gap-3 rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 sm:grid-cols-[1fr_auto]">
							<div class="min-w-0">
								<p class="text-sm text-zinc-400">Join token expires {formatTime(joinTokenExpiresAt)}</p>
								<p class="mt-1 break-all font-mono text-sm text-emerald-300">{joinToken}</p>
							</div>
							<Button
								type="button"
								variant="outline"
								size="sm"
								onclick={copyJoinToken}
							>
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
										onclick={() => revokeJoinToken(token.id)}
									>
										Revoke
									</Button>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if loading}
					<CardContent class="py-8 text-sm text-zinc-400">Loading nodes...</CardContent>
				{:else if nodes.length === 0}
					<CardContent class="py-8 text-sm text-zinc-400">No nodes have connected yet.</CardContent>
				{:else}
					<div class="divide-y divide-zinc-800">
						{#each nodes as node (node.id)}
							<button
								type="button"
								class={`grid w-full gap-3 px-4 py-4 text-left hover:bg-zinc-800/70 sm:grid-cols-[1fr_auto] ${node.id === selectedNodeID ? 'bg-zinc-800' : ''}`}
								onclick={() => selectNode(node.id)}
							>
								<div class="min-w-0">
									<div class="flex items-center gap-2">
										<span class="truncate font-medium text-white">{node.name || node.id}</span>
										<Badge variant={node.connected ? 'success' : 'default'}>
											{node.connected ? 'connected' : 'offline'}
										</Badge>
									</div>
									<p class="mt-1 truncate text-sm text-zinc-400">{node.id}</p>
								</div>
								<div class="text-sm text-zinc-400">Last seen {formatTime(node.last_seen)}</div>
							</button>
						{/each}
					</div>
				{/if}
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Command History</CardTitle>
				</CardHeader>
				{#if commands.length === 0}
					<CardContent class="py-8 text-sm text-zinc-400">No commands sent this session.</CardContent>
				{:else}
					<div class="divide-y divide-zinc-800">
						{#each commands as command (command.id)}
							<div class="grid gap-2 px-4 py-3 sm:grid-cols-[1fr_auto]">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium text-white">{command.action} -> {command.target_id || '-'}</p>
									<p class="mt-1 truncate text-xs text-zinc-500">{command.id}</p>
									{#if command.message}
										<p class="mt-1 text-sm text-zinc-300">{command.message}</p>
									{/if}
								</div>
								<Badge
									class="h-fit"
									variant={command.status === 'pending'
										? 'warning'
										: command.status === 'succeeded'
											? 'success'
											: 'destructive'}
								>
									{command.status}
								</Badge>
							</div>
						{/each}
					</div>
				{/if}
			</Card>
		</section>

		<aside class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle>Containers</CardTitle>
					<p class="mt-1 text-sm text-zinc-400">
						{#if selectedNode}
							Target node: {selectedNode.id}
						{:else}
							Select a connected node.
						{/if}
					</p>
				</CardHeader>

				<CardContent>
					{#if error}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{error}
						</div>
					{/if}

					<form class="mt-4 space-y-4" method="POST" use:enhanceCreate>
						<div>
							<Label for="container-name">Name</Label>
							<Input
								id="container-name"
								name="name"
								bind:value={$createForm.name}
								autocomplete="off"
								aria-invalid={$createErrors.name ? 'true' : undefined}
								{...$createConstraints.name}
							/>
							{#if $createErrors.name}
								<p class="mt-1 text-sm text-red-300">{$createErrors.name[0]}</p>
							{/if}
						</div>
						<div>
							<Label for="container-image">Image</Label>
							<Input
								id="container-image"
								name="image"
								bind:value={$createForm.image}
								autocomplete="off"
								aria-invalid={$createErrors.image ? 'true' : undefined}
								{...$createConstraints.image}
							/>
							{#if $createErrors.image}
								<p class="mt-1 text-sm text-red-300">{$createErrors.image[0]}</p>
							{/if}
						</div>
						<Button type="submit" class="w-full" disabled={busy || createSubmitting || !selectedNodeID}>
							<Plus size={16} />
							Create
						</Button>
					</form>

					<div class="mt-6 border-t border-zinc-800 pt-4">
						<div class="mb-3 flex items-center justify-between gap-3">
							<p class="text-sm font-medium text-zinc-200">Known containers</p>
							<Button
								type="button"
								variant="outline"
								size="sm"
								disabled={!selectedNodeID || containersLoading}
								onclick={() => void loadContainers()}
							>
								<RefreshCw size={14} />
								Refresh
							</Button>
						</div>

						{#if containersLoading}
							<p class="py-4 text-sm text-zinc-400">Loading containers...</p>
						{:else if !selectedNodeID}
							<p class="py-4 text-sm text-zinc-400">Select a node first.</p>
						{:else if containers.length === 0}
							<p class="py-4 text-sm text-zinc-400">No containers known for this node.</p>
						{:else}
							<div class="max-h-72 divide-y divide-zinc-800 overflow-auto rounded-md border border-zinc-800">
								{#each containers as container (container.id)}
									<button
										type="button"
										class={`grid w-full gap-2 px-3 py-3 text-left hover:bg-zinc-800/70 ${container.id === selectedContainerID ? 'bg-zinc-800' : 'bg-zinc-950'}`}
										onclick={() => (selectedContainerID = container.id)}
									>
										<div class="flex items-center justify-between gap-3">
											<span class="truncate text-sm font-medium text-white">{container.name || container.id}</span>
											<Badge variant={containerStatusVariant(container.status)}>
												{container.status || 'unknown'}
											</Badge>
										</div>
										{#if container.image}
											<p class="truncate text-xs text-zinc-400">{container.image}</p>
										{/if}
										<p class="truncate font-mono text-xs text-zinc-500">{container.id}</p>
									</button>
								{/each}
							</div>
						{/if}

						{#if selectedContainer}
							<div class="mt-4 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-3">
								<div class="flex items-center justify-between gap-3">
									<div class="min-w-0">
										<p class="truncate text-sm font-medium text-white">{selectedContainer.name || selectedContainer.id}</p>
										<p class="mt-1 truncate font-mono text-xs text-zinc-500">{selectedContainer.id}</p>
									</div>
									<Badge variant={containerStatusVariant(selectedContainer.status)}>
										{selectedContainer.status || 'unknown'}
									</Badge>
								</div>
							</div>
						{/if}

						<div class="mt-3 grid grid-cols-3 gap-2">
							<Button
								type="button"
								variant="outline"
								disabled={busy || !selectedNodeID || !selectedContainer}
								onclick={() => runContainerAction('start')}
							>
								<Play size={15} />
								Start
							</Button>
							<Button
								type="button"
								variant="outline"
								disabled={busy || !selectedNodeID || !selectedContainer}
								onclick={() => runContainerAction('stop')}
							>
								<Square size={15} />
								Stop
							</Button>
							<Button
								type="button"
								variant="destructive"
								disabled={busy || !selectedNodeID || !selectedContainer}
								onclick={() => runContainerAction('remove')}
							>
								<Trash2 size={15} />
								Remove
							</Button>
						</div>
					</div>
				</CardContent>
			</Card>
		</aside>
	</div>
</main>
