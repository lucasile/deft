<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, LogOut, RefreshCw } from '@lucide/svelte';
	import { auth, panel, type Command } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';

	let commands = $state<Command[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	const highlightedCommandID = $derived(page.url.searchParams.get('command') ?? '');

	onMount(() => {
		void loadCommands();

		const events = panel.events();
		events.addEventListener('command.updated', () => {
			void loadCommands({ quiet: true });
		});
		return () => events.close();
	});

	const loadCommands = async (options: { quiet?: boolean } = {}) => {
		if (!options.quiet) {
			loading = true;
			error = null;
		}
		try {
			commands = await panel.commands(100);
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

	const formatTime = (seconds: number) => {
		if (!seconds) return 'Never';
		return new Date(seconds * 1000).toLocaleString();
	};

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};
</script>

<svelte:head>
	<title>Command History - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => goto('/')}>
					<ArrowLeft size={16} />
					Nodes
				</Button>
				<h1 class="text-xl font-semibold tracking-normal text-white">Command History</h1>
				<p class="mt-1 text-sm text-zinc-400">Recent panel actions and agent results</p>
			</div>
			<div class="flex gap-2">
				<Button type="button" variant="outline" disabled={loading} onclick={() => void loadCommands()}>
					<RefreshCw size={16} />
					Refresh
				</Button>
				<Button type="button" variant="outline" onclick={logout}>
					<LogOut size={16} />
					Log out
				</Button>
			</div>
		</div>
	</header>

	<div class="mx-auto max-w-6xl px-4 py-6 sm:px-6 lg:px-8">
		<Card>
			<CardHeader>
				<CardTitle>Commands</CardTitle>
			</CardHeader>
			{#if error}
				<CardContent class="border-t border-zinc-800">
					<div class="rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
						{error}
					</div>
				</CardContent>
			{/if}
			{#if loading}
				<CardContent class="py-8 text-sm text-zinc-400">Loading command history...</CardContent>
			{:else if commands.length === 0}
				<CardContent class="py-8 text-sm text-zinc-400">No commands recorded yet.</CardContent>
			{:else}
				<div class="divide-y divide-zinc-800">
					{#each commands as command (command.id)}
						<div class={`grid gap-3 px-4 py-4 sm:grid-cols-[1fr_auto] ${command.id === highlightedCommandID ? 'bg-zinc-800' : ''}`}>
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-white">{command.action} -> {command.target_id || '-'}</p>
								<p class="mt-1 truncate font-mono text-xs text-zinc-500">{command.id}</p>
								<p class="mt-1 truncate text-xs text-zinc-500">{command.node_id} · {formatTime(command.created_at)}</p>
								{#if command.message}
									<pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-zinc-800 bg-zinc-950 p-3 text-xs text-zinc-300">{command.message}</pre>
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
	</div>
</main>
