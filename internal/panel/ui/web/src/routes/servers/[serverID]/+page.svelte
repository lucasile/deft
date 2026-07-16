<script lang="ts">
	import { tick } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Box, Clock3, Container, LogOut, Play, RefreshCw, Send, Settings, Square, Trash2, ServerIcon } from '@lucide/svelte';
	import { auth, panel, type LogChunkPayload, type PanelEventPayload, type Server } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { backOrGoto } from '$lib/navigation';
	import {
		serverCanRemove,
		serverCanRestart,
		serverCanStart,
		serverCanStop,
		serverState,
		serverStateLabel,
		serverStateVariant,
	} from '$lib/server-status';

	let server = $state<Server | null>(null);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state<string | null>(null);
	let logText = $state('');
	let logLoading = $state(false);
	let logError = $state<string | null>(null);
	let logLive = $state(false);
	let consoleCommand = $state('');
	let consoleBusy = $state(false);
	let consoleMessage = $state('');
	let consoleError = $state<string | null>(null);
	let consoleCommandID = $state('');
	let localServerStatus = $state<string | null>(null);
	let pendingActionCommandID = $state('');
	let confirmAction = $state<'remove-server' | null>(null);
	let logOutputElement = $state<HTMLPreElement | null>(null);
	let consoleInputElement = $state<HTMLInputElement | null>(null);
	let logEvents: EventSource | null = null;
	let logDecoder = new TextDecoder();
	let autoStartedLogForStatus = '';

	const serverID = $derived(page.params.serverID);
	const displayStatus = $derived(localServerStatus || server?.status || '');
	const canStart = $derived(serverCanStart(server, displayStatus));
	const canStop = $derived(serverCanStop(server, displayStatus));
	const canRestart = $derived(serverCanRestart(server, displayStatus));
	const canRemove = $derived(serverCanRemove(server, displayStatus));
	const canSendConsoleCommand = $derived(Boolean(server?.container_id && serverState(displayStatus) === 'online'));

	onMount(() => {
		void loadInitialData();

		const events = panel.events();
		events.addEventListener('containers.changed', (event) => {
			const payload = parseEventPayload(event);
			if (!server || !payload.node_id || payload.node_id === server.node_id) {
				void loadServer({ quiet: true });
			}
		});
		events.addEventListener('command.updated', (event) => {
			const payload = parseEventPayload(event);
			if (payload.command_id && payload.command_id === pendingActionCommandID) {
				pendingActionCommandID = '';
			}
			if (payload.command_id && payload.command_id === consoleCommandID) {
				void loadConsoleCommandResult(payload.command_id);
			}
			void loadServer({ quiet: true });
		});
		return () => {
			events.close();
			stopLiveLogs();
		};
	});

	const loadInitialData = async () => {
		await loadServer();
		if (!logEvents && !logLoading) {
			startLiveLogs();
		}
	};

	const loadServer = async (options: { quiet?: boolean } = {}) => {
		if (!serverID) return;
		if (!options.quiet) {
			loading = true;
			error = null;
		}
		try {
			const nextServer = await panel.server(serverID);
			server = nextServer;
			reconcileLocalServerStatus(nextServer);
			maybeStartLogsForRunningServer(nextServer);
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

	const runServerAction = async (action: 'start' | 'stop' | 'restart' | 'remove') => {
		if (!server || !actionAllowed(action)) return;
		busy = true;
		error = null;
		const previousStatus = localServerStatus;
		localServerStatus = requestedStatus(action);
		try {
			if (action === 'start' || action === 'restart') {
				autoStartedLogForStatus = '';
			}
			const response = await panel.serverAction(server.id, action);
			pendingActionCommandID = response.command_id;
			if (action === 'remove') {
				stopLiveLogs();
				await goto('/');
				return;
			}
			await loadServer({ quiet: true });
		} catch (err) {
			localServerStatus = previousStatus;
			pendingActionCommandID = '';
			error = cleanError(err);
		} finally {
			busy = false;
			confirmAction = null;
		}
	};

	const requestedStatus = (action: 'start' | 'stop' | 'restart' | 'remove') => {
		if (action === 'start') return 'starting';
		if (action === 'stop') return 'stopping';
		if (action === 'restart') return 'restarting';
		return 'removing';
	};

	const actionAllowed = (action: 'start' | 'stop' | 'restart' | 'remove') => {
		if (action === 'start') return canStart;
		if (action === 'stop') return canStop;
		if (action === 'restart') return canRestart;
		return canRemove;
	};

	const reconcileLocalServerStatus = (nextServer: Server) => {
		if (!localServerStatus) return;
		if (serverState(nextServer.status) !== serverState(localServerStatus)) {
			localServerStatus = null;
			pendingActionCommandID = '';
		}
	};

	const maybeStartLogsForRunningServer = (nextServer: Server) => {
		if (!nextServer.node_id || !nextServer.container_id || serverState(nextServer.status) !== 'online') return;
		if (logEvents || logLoading || logLive) return;

		const statusKey = `${nextServer.container_id}:running`;
		if (autoStartedLogForStatus === statusKey) return;
		autoStartedLogForStatus = statusKey;
		void startLiveLogs();
	};

	const startLiveLogs = async () => {
		if (!server?.node_id || !server.container_id) return;
		stopLiveLogs();
		logText = '';
		logError = null;
		logLoading = true;
		logLive = false;
		logDecoder = new TextDecoder();

		let streamID = '';
		try {
			const response = await panel.createContainerLogStream(server.node_id, server.container_id);
			streamID = response.stream_id;
		} catch (err) {
			logLoading = false;
			logError = cleanError(err);
			return;
		}

		const stream = panel.containerLogStream(server.node_id, server.container_id, streamID);
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

	const sendConsoleCommand = async () => {
		if (!server || !canSendConsoleCommand || consoleBusy) return;

		const command = consoleCommand.trim();
		if (!command) {
			consoleError = 'Command is required.';
			consoleMessage = '';
			return;
		}

		consoleBusy = true;
		consoleError = null;
		consoleMessage = 'Sending command...';
		try {
			const response = await panel.serverConsoleCommand(server.id, command);
			consoleCommandID = response.command_id;
			consoleCommand = '';
			consoleMessage = 'Command sent.';
			logText += `${logText.endsWith('\n') || logText === '' ? '' : '\n'}> ${command}\n`;
			void scrollLogsToBottom();
		} catch (err) {
			consoleCommandID = '';
			consoleMessage = '';
			consoleError = cleanError(err);
		} finally {
			consoleBusy = false;
			void refocusConsoleInput();
		}
	};

	const refocusConsoleInput = async () => {
		await tick();
		window.requestAnimationFrame(() => {
			window.setTimeout(() => {
				if (!consoleInputElement || consoleInputElement.disabled) return;
				consoleInputElement.focus({ preventScroll: true });
				const cursorPosition = consoleInputElement.value.length;
				consoleInputElement.setSelectionRange(cursorPosition, cursorPosition);
			}, 0);
		});
	};

	const loadConsoleCommandResult = async (commandID: string) => {
		try {
			const command = await panel.command(commandID);
			consoleCommandID = '';
			if (command.status === 'failed') {
				consoleMessage = '';
				consoleError = command.message || 'Command failed.';
				return;
			}
			consoleError = null;
			consoleMessage = command.message || 'Command sent.';
		} catch (err) {
			consoleError = cleanError(err);
		}
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

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
	};

	const openNode = () => {
		if (!server) return;
		goto(`/nodes/${server.node_id}`);
	};

	const openContainer = () => {
		if (!server?.container_id) return;
		goto(`/nodes/${server.node_id}/containers/${server.container_id}?from=${encodeURIComponent(`/servers/${serverID}`)}`);
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
				<Button type="button" variant="outline" onclick={() => goto(`/servers/${serverID}/config`)}>
					<Settings size={16} />
					Config
				</Button>
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
							<Badge variant={serverStateVariant(displayStatus)}>{serverStateLabel(displayStatus)}</Badge>
						</div>

						<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
							<Button type="button" variant="outline" disabled={busy || !canStart} onclick={() => runServerAction('start')}>
								<Play size={15} />
								Start
							</Button>
							<Button type="button" variant="outline" disabled={busy || !canRestart} onclick={() => runServerAction('restart')}>
								<RefreshCw size={15} />
								Restart
							</Button>
							<Button type="button" variant="outline" disabled={busy || !canStop} onclick={() => runServerAction('stop')}>
								<Square size={15} />
								Stop
							</Button>
							<Button
								type="button"
								variant="destructive"
								disabled={busy || !canRemove}
								onclick={() => (confirmAction = 'remove-server')}
							>
								<Trash2 size={15} />
								Remove
							</Button>
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

		<section class="space-y-6">
			<Card>
				<CardHeader class="flex flex-row items-center justify-between">
					<div>
						<CardTitle>Console</CardTitle>
						<p class="text-sm text-zinc-400">{logLive ? 'Live' : canSendConsoleCommand ? 'Ready' : 'Server must be online'}</p>
					</div>
					<Button type="button" variant="outline" size="sm" disabled={busy || !server?.container_id} onclick={startLiveLogs}>
						<RefreshCw size={14} />
						Reconnect
					</Button>
				</CardHeader>
				<CardContent class="space-y-3">
					<div class="rounded-md border border-zinc-800 bg-zinc-950">
						{#if !server?.container_id}
							<p class="px-3 py-8 text-sm text-zinc-400">Console will appear after the server container is created.</p>
						{:else if logLoading}
							<p class="px-3 py-8 text-sm text-zinc-400">Connecting console...</p>
						{:else if logError}
							<div class="m-3 rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
								{logError}
							</div>
						{/if}

						<pre
							bind:this={logOutputElement}
							class="max-h-[32rem] min-h-80 overflow-auto whitespace-pre-wrap p-3 text-xs text-zinc-300"
						>{logText || (!logLoading && !logError && server?.container_id ? 'No console output yet.' : '')}</pre>

						<form
							class="grid gap-2 border-t border-zinc-800 p-3 sm:grid-cols-[1fr_auto]"
							onsubmit={(event) => {
								event.preventDefault();
								void sendConsoleCommand();
							}}
						>
							<input
								bind:this={consoleInputElement}
								class="h-10 w-full rounded-md border border-zinc-700 bg-black px-3 font-mono text-sm text-zinc-100 outline-none focus:border-emerald-500 disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="say Hello from Deft"
								maxlength="512"
								bind:value={consoleCommand}
								disabled={consoleBusy || !canSendConsoleCommand}
							/>
							<Button
								type="submit"
								disabled={consoleBusy || !canSendConsoleCommand || !consoleCommand.trim()}
								onpointerdown={(event) => event.preventDefault()}
							>
								<Send size={15} />
								{consoleBusy ? 'Sending...' : 'Send'}
							</Button>
						</form>
					</div>

					{#if consoleError}
						<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
							{consoleError}
						</div>
					{/if}
					{#if consoleMessage}
						<div class="rounded-md border border-emerald-900/60 bg-emerald-950/50 px-3 py-2 text-sm text-emerald-200">
							{consoleMessage}
						</div>
					{/if}
				</CardContent>
			</Card>

		</section>
	</div>

	<ConfirmDialog
		bind:open={() => confirmAction === 'remove-server', (value) => {
			if (!value) confirmAction = null;
		}}
		title="Remove server?"
		description={`Remove "${server?.name || 'this server'}" from its node? This deletes the backing container and its volumes.`}
		confirmLabel="Remove"
		onconfirm={() => runServerAction('remove')}
	/>
</main>
