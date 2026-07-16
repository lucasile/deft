<script lang="ts">
	import { tick } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Clock3, LogOut, Play, RefreshCw, Square, Trash2 } from '@lucide/svelte';
	import { auth, panel, type Container, type LogChunkPayload, type Node, type PanelEventPayload } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { backOrGoto } from '$lib/navigation';

	let nodes = $state<Node[]>([]);
	let containers = $state<Container[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state<string | null>(null);
	let logText = $state('');
	let logLoading = $state(false);
	let logError = $state<string | null>(null);
	let logLive = $state(false);
	let confirmAction = $state<'remove-container' | null>(null);
	let localContainerStatus = $state<string | null>(null);
	let pendingActionCommandID = $state('');
	let logOutputElement = $state<HTMLPreElement | null>(null);
	let logEvents: EventSource | null = null;
	let logDecoder = new TextDecoder();
	let autoStartedLogForStatus = '';

	const nodeID = $derived(page.params.nodeID);
	const containerID = $derived(page.params.containerID);
	const selectedNode = $derived(nodes.find((node) => node.id === nodeID));
	const container = $derived(containers.find((item) => item.id === containerID));
	const canAct = $derived(Boolean(selectedNode?.connected && container));
	const displayStatus = $derived(localContainerStatus || container?.status || '');
	const actionPending = $derived(displayStatus.endsWith('_requested'));
	const canStart = $derived(Boolean(canAct && !actionPending && displayStatus !== 'running'));
	const canStop = $derived(Boolean(canAct && !actionPending && displayStatus === 'running'));
	const canRemove = $derived(Boolean(canAct && !actionPending));

	onMount(() => {
		void loadInitialData();

		const events = panel.events();
		events.addEventListener('nodes.changed', () => {
			void loadNodes({ quiet: true });
		});
		events.addEventListener('containers.changed', (event) => {
			const payload = parseEventPayload(event);
			if (!payload.node_id || payload.node_id === nodeID) {
				void loadContainers({ quiet: true });
			}
		});
		events.addEventListener('command.updated', (event) => {
			const payload = parseEventPayload(event);
			if (payload.command_id && payload.command_id === pendingActionCommandID) {
				pendingActionCommandID = '';
			}
			void loadContainers({ quiet: true });
		});
		return () => {
			events.close();
			stopLiveLogs();
		};
	});

	const loadInitialData = async () => {
		await loadAll();
		if (!logEvents && !logLoading) {
			startLiveLogs();
		}
	};

	const loadAll = async () => {
		loading = true;
		error = null;
		await Promise.all([loadNodes({ quiet: true }), loadContainers({ quiet: true })]);
		loading = false;
	};

	const loadNodes = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) error = null;
		try {
			nodes = await panel.nodes();
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		}
	};

	const loadContainers = async (options: { quiet?: boolean } = {}) => {
		if (!nodeID) return;
		if (!options.quiet) error = null;
		try {
			const nextContainers = await panel.containers(nodeID);
			containers = nextContainers;
			reconcileLocalContainerStatus(nextContainers);
			maybeStartLogsForRunningContainer(nextContainers);
		} catch (err) {
			error = cleanError(err);
		}
	};

	const runContainerAction = async (action: 'start' | 'stop' | 'remove') => {
		if (!nodeID || !containerID || !canAct) return;
		busy = true;
		error = null;
		const previousStatus = localContainerStatus;
		localContainerStatus = requestedStatus(action);
		try {
			if (action === 'start') {
				autoStartedLogForStatus = '';
			}
			const response = await panel.containerAction(nodeID, containerID, action);
			pendingActionCommandID = response.command_id;
			if (action === 'remove') {
				goto(`/nodes/${nodeID}`);
				return;
			}
			await loadContainers({ quiet: true });
		} catch (err) {
			localContainerStatus = previousStatus;
			pendingActionCommandID = '';
			error = cleanError(err);
		} finally {
			busy = false;
		}
	};

	const requestedStatus = (action: 'start' | 'stop' | 'remove') => {
		if (action === 'start') return 'start_requested';
		if (action === 'stop') return 'stop_requested';
		return 'remove_requested';
	};

	const reconcileLocalContainerStatus = (nextContainers: Container[]) => {
		if (!localContainerStatus) return;
		const nextContainer = nextContainers.find((item) => item.id === containerID);
		if (!nextContainer) {
			localContainerStatus = null;
			pendingActionCommandID = '';
			return;
		}
		if (nextContainer.status && nextContainer.status !== localContainerStatus && !nextContainer.status.endsWith('_requested')) {
			localContainerStatus = null;
			pendingActionCommandID = '';
		}
	};

	const maybeStartLogsForRunningContainer = (nextContainers: Container[]) => {
		const nextContainer = nextContainers.find((item) => item.id === containerID);
		if (!nodeID || !containerID || !selectedNode?.connected || !nextContainer) return;
		if (nextContainer.status !== 'running') return;
		if (logEvents || logLoading || logLive) return;

		const statusKey = `${nextContainer.id}:running`;
		if (autoStartedLogForStatus === statusKey) return;
		autoStartedLogForStatus = statusKey;
		void startLiveLogs();
	};

	const startLiveLogs = async () => {
		if (!nodeID || !containerID || !canAct) return;
		stopLiveLogs();
		logText = '';
		logError = null;
		logLoading = true;
		logLive = false;
		logDecoder = new TextDecoder();

		let streamID = '';
		try {
			const response = await panel.createContainerLogStream(nodeID, containerID);
			streamID = response.stream_id;
		} catch (err) {
			logLoading = false;
			logError = cleanError(err);
			return;
		}

		const stream = panel.containerLogStream(nodeID, containerID, streamID);
		logEvents = stream;
		stream.addEventListener('ready', () => {
			logLoading = false;
			logLive = true;
		});
		stream.addEventListener('logs.chunk', (event) => {
			const payload = parseLogChunk(event);
			if (!payload) return;
			if (payload.error) {
				logError = payload.error;
				logLive = false;
			}
			if (payload.data) {
				logText += decodeBase64Chunk(payload.data, !payload.eof);
				void scrollLogsToBottom();
			}
			if (payload.eof) {
				logText += logDecoder.decode();
				logLive = false;
				logLoading = false;
				stream.close();
				if (logEvents === stream) {
					logEvents = null;
				}
			}
		});
		stream.onerror = () => {
			logLoading = false;
			logLive = false;
			if (!logText && !logError) {
				logError = 'Log stream disconnected.';
			}
			stream.close();
			if (logEvents === stream) {
				logEvents = null;
			}
		};
	};

	const stopLiveLogs = () => {
		if (logEvents) {
			logEvents.close();
			logEvents = null;
		}
		logLive = false;
	};

	const scrollLogsToBottom = async () => {
		await tick();
		if (logOutputElement) {
			logOutputElement.scrollTop = logOutputElement.scrollHeight;
		}
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
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

	const parseLogChunk = (event: Event): LogChunkPayload | null => {
		if (!(event instanceof MessageEvent) || typeof event.data !== 'string') return null;

		try {
			return JSON.parse(event.data) as LogChunkPayload;
		} catch {
			return null;
		}
	};

	const decodeBase64Chunk = (value: string, streaming: boolean) => {
		const binary = atob(value);
		const bytes = new Uint8Array(binary.length);
		for (let index = 0; index < binary.length; index += 1) {
			bytes[index] = binary.charCodeAt(index);
		}
		return logDecoder.decode(bytes, { stream: streaming });
	};
</script>

<svelte:head>
	<title>{container?.name || containerID} - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => backOrGoto(`/nodes/${nodeID}`)}>
					<ArrowLeft size={16} />
					Back
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">{container?.name || containerID}</h1>
				<p class="mt-1 truncate text-sm text-zinc-400">{selectedNode?.name || nodeID}</p>
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
					<CardTitle>Container</CardTitle>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if error}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{error}
						</div>
					{/if}
					{#if loading}
						<p class="text-sm text-zinc-400">Loading container...</p>
					{:else if !container}
						<p class="text-sm text-zinc-400">Container not found.</p>
					{:else}
						<div class="space-y-2">
							<div class="flex items-center justify-between gap-3">
								<p class="truncate text-sm font-medium text-white">{container.name || container.id}</p>
								<Badge variant={containerStatusVariant(displayStatus)}>{displayStatus || 'unknown'}</Badge>
							</div>
							{#if container.image}
								<p class="truncate text-sm text-zinc-400">{container.image}</p>
							{/if}
							<p class="break-all font-mono text-xs text-zinc-500">{container.id}</p>
						</div>
						<div class="grid grid-cols-3 gap-2">
							<Button type="button" variant="outline" disabled={busy || !canStart} onclick={() => runContainerAction('start')}>
								<Play size={15} />
								Start
							</Button>
							<Button type="button" variant="outline" disabled={busy || !canStop} onclick={() => runContainerAction('stop')}>
								<Square size={15} />
								Stop
							</Button>
							<Button
								type="button"
								variant="destructive"
								disabled={busy || !canRemove}
								onclick={() => (confirmAction = 'remove-container')}
							>
								<Trash2 size={15} />
								Remove
							</Button>
						</div>
					{/if}
				</CardContent>
			</Card>
		</section>

		<section>
			<Card>
				<CardHeader class="flex flex-row items-center justify-between">
					<div>
						<CardTitle>Logs</CardTitle>
						<p class="text-sm text-zinc-400">{logLive ? 'Live' : 'Not connected'}</p>
					</div>
					<Button type="button" variant="outline" size="sm" disabled={busy || !canAct} onclick={startLiveLogs}>
						<RefreshCw size={14} />
						Restart logs
					</Button>
				</CardHeader>
				<CardContent>
					{#if logLoading}
						<p class="py-8 text-sm text-zinc-400">Loading recent logs...</p>
					{:else if logError}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{logError}
						</div>
					{:else}
						<pre
							bind:this={logOutputElement}
							class="max-h-[34rem] overflow-auto whitespace-pre-wrap rounded-md border border-zinc-800 bg-zinc-950 p-3 text-xs text-zinc-300"
						>{logText || 'No logs available.'}</pre>
					{/if}
				</CardContent>
			</Card>
		</section>
	</div>

	<ConfirmDialog
		bind:open={() => confirmAction === 'remove-container', (value) => {
			if (!value) confirmAction = null;
		}}
		title="Remove container?"
		description={`Remove "${container?.name || container?.id || 'this container'}" from this agent? This deletes the container and its volumes.`}
		confirmLabel="Remove"
		onconfirm={() => runContainerAction('remove')}
	/>
</main>
