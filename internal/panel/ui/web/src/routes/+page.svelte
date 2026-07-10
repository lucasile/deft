<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { LogOut, Play, Plus, RefreshCw, Square, Trash2 } from '@lucide/svelte';
	import { defaults, superForm } from 'sveltekit-superforms';
	import { zod4 } from 'sveltekit-superforms/adapters';
	import { auth, panel, type Command, type Node } from '$lib/api/client';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { createContainerSchema } from '$lib/schemas';

	let nodes = $state<Node[]>([]);
	let selectedNodeID = $state('');
	let containerID = $state('hello-nginx');
	let loading = $state(true);
	let busy = $state(false);
	let createSubmitting = $state(false);
	let error = $state<string | null>(null);
	let commands = $state<Command[]>([]);

	const connectedNodes = $derived(nodes.filter((node) => node.connected));
	const selectedNode = $derived(nodes.find((node) => node.id === selectedNodeID));
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
					containerID = form.data.name;
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
		void loadNodes();
	});

	const loadNodes = async () => {
		loading = true;
		error = null;
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
			loading = false;
		}
	};

	const runContainerAction = async (action: 'start' | 'stop' | 'remove') => {
		if (!selectedNodeID || !containerID) return;

		await runCommand(async () => {
			const response = await panel.containerAction(selectedNodeID, containerID, action);
			await trackCommand(response.command_id);
		});
	};

	const runCommand = async (operation: () => Promise<void>) => {
		busy = true;
		error = null;
		try {
			await operation();
			await loadNodes();
		} catch (err) {
			error = cleanError(err);
		} finally {
			busy = false;
		}
	};

	const trackCommand = async (commandID: string) => {
		let command = await panel.command(commandID);
		commands = [command, ...commands.filter((item) => item.id !== command.id)].slice(0, 8);

		for (let attempt = 0; attempt < 10 && command.status === 'pending'; attempt++) {
			await delay(500);
			command = await panel.command(commandID);
			commands = [command, ...commands.filter((item) => item.id !== command.id)].slice(0, 8);
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

	const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
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
					<Button
						type="button"
						variant="outline"
						size="sm"
						disabled={loading}
						onclick={loadNodes}
					>
						<RefreshCw size={15} />
						Refresh
					</Button>
				</CardHeader>

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
								onclick={() => (selectedNodeID = node.id)}
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
					<CardTitle>Container Action</CardTitle>
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

					<div class="mt-6 space-y-4 border-t border-zinc-800 pt-4">
						<div>
							<Label for="container-id">Container ID or name</Label>
							<Input id="container-id" bind:value={containerID} autocomplete="off" />
						</div>
						<div class="grid grid-cols-3 gap-2">
							<Button
								type="button"
								variant="outline"
								disabled={busy || !selectedNodeID || !containerID}
								onclick={() => runContainerAction('start')}
							>
								<Play size={15} />
								Start
							</Button>
							<Button
								type="button"
								variant="outline"
								disabled={busy || !selectedNodeID || !containerID}
								onclick={() => runContainerAction('stop')}
							>
								<Square size={15} />
								Stop
							</Button>
							<Button
								type="button"
								variant="destructive"
								disabled={busy || !selectedNodeID || !containerID}
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
