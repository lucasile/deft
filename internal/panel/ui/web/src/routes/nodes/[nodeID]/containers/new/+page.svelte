<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Clock3, LogOut, Plus } from '@lucide/svelte';
	import { defaults, superForm } from 'sveltekit-superforms';
	import { zod4 } from 'sveltekit-superforms/adapters';
	import { auth, panel, type Container, type Node } from '$lib/api/client';
	import { parseCreateConfig } from '$lib/container-config';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { createContainerSchema } from '$lib/schemas';

	let nodes = $state<Node[]>([]);
	let loading = $state(true);
	let createSubmitting = $state(false);
	let error = $state<string | null>(null);

	const nodeID = $derived(page.params.nodeID);
	const selectedNode = $derived(nodes.find((node) => node.id === nodeID));

	const createContainerForm = superForm(
		defaults(
			{
				name: 'minecraft-1',
				image: 'itzg/minecraft-server:latest',
				ports: '25565:25565/tcp',
				env: 'EULA=TRUE',
				volumes: '/var/lib/deft/volumes/minecraft-1:/data',
				restart_policy: 'unless-stopped' as const,
			},
			zod4(createContainerSchema),
		),
		{
			SPA: true,
			validators: zod4(createContainerSchema),
			async onUpdate({ form }) {
				if (!form.valid || createSubmitting || !nodeID) return;

				createSubmitting = true;
				error = null;
				try {
					const config = parseCreateConfig(form.data);
					const response = await panel.createContainer(nodeID, form.data.name, form.data.image, config);
					savePendingCreate({
						id: response.command_id,
						node_id: nodeID,
						name: form.data.name,
						image: form.data.image,
						status: 'create_requested',
					});
					goto(`/nodes/${nodeID}`);
				} catch (err) {
					error = cleanError(err);
				} finally {
					createSubmitting = false;
				}
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
		} catch (err) {
			error = cleanError(err);
			if (error.includes('missing session') || error.includes('invalid session')) {
				goto('/login');
			}
		} finally {
			loading = false;
		}
	};

	const savePendingCreate = (container: Container) => {
		if (!nodeID) return;
		const storageKey = pendingCreateStorageKey(nodeID);
		const rawValue = sessionStorage.getItem(storageKey);
		let pending: Container[] = [];
		if (rawValue) {
			try {
				pending = JSON.parse(rawValue) as Container[];
			} catch {
				pending = [];
			}
		}
		sessionStorage.setItem(storageKey, JSON.stringify([...pending, container]));
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};

	const pendingCreateStorageKey = (id: string) => `deft.pending-creates.${id}`;
</script>

<svelte:head>
	<title>Create Server - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => goto(`/nodes/${nodeID}`)}>
					<ArrowLeft size={16} />
					Agent
				</Button>
				<h1 class="truncate text-xl font-semibold tracking-normal text-white">Create server</h1>
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

	<div class="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
		<Card>
			<CardHeader>
				<CardTitle>Server Settings</CardTitle>
			</CardHeader>
			<CardContent>
				{#if error}
					<div class="mb-4 rounded-md border border-red-900/60 bg-red-950/60 px-3 py-2 text-sm text-red-200">
						{error}
					</div>
				{/if}
				{#if loading}
					<p class="text-sm text-zinc-400">Loading agent...</p>
				{:else if !selectedNode}
					<p class="text-sm text-zinc-400">Agent not found.</p>
				{:else if !selectedNode.connected}
					<p class="text-sm text-zinc-400">Start the agent before creating servers.</p>
				{:else}
					<form class="space-y-5" method="POST" use:enhanceCreate>
						<div class="grid gap-4 md:grid-cols-2">
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
						</div>

						<div>
							<Label for="container-ports">Ports</Label>
							<textarea
								id="container-ports"
								name="ports"
								bind:value={$createForm.ports}
								class="mt-2 min-h-20 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
								placeholder="25565:25565/tcp"
							></textarea>
							<p class="mt-1 text-xs text-zinc-500">One per line: host:container/protocol.</p>
							{#if $createErrors.ports}
								<p class="mt-1 text-sm text-red-300">{$createErrors.ports[0]}</p>
							{/if}
						</div>

						<div>
							<Label for="container-env">Environment</Label>
							<textarea
								id="container-env"
								name="env"
								bind:value={$createForm.env}
								class="mt-2 min-h-24 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
								placeholder="EULA=TRUE"
							></textarea>
							<p class="mt-1 text-xs text-zinc-500">One per line: KEY=value.</p>
							{#if $createErrors.env}
								<p class="mt-1 text-sm text-red-300">{$createErrors.env[0]}</p>
							{/if}
						</div>

						<div>
							<Label for="container-volumes">Volumes</Label>
							<textarea
								id="container-volumes"
								name="volumes"
								bind:value={$createForm.volumes}
								class="mt-2 min-h-20 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
								placeholder="/var/lib/deft/volumes/minecraft-1:/data"
							></textarea>
							<p class="mt-1 text-xs text-zinc-500">One per line: host:container[:ro]. Host path must be under /var/lib/deft/volumes.</p>
							{#if $createErrors.volumes}
								<p class="mt-1 text-sm text-red-300">{$createErrors.volumes[0]}</p>
							{/if}
						</div>

						<div>
							<Label for="container-restart-policy">Restart policy</Label>
							<select
								id="container-restart-policy"
								name="restart_policy"
								bind:value={$createForm.restart_policy}
								class="mt-2 h-10 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 text-sm text-zinc-100 outline-none focus:border-zinc-500"
							>
								<option value="no">No restart</option>
								<option value="unless-stopped">Unless stopped</option>
								<option value="on-failure">On failure</option>
								<option value="always">Always</option>
							</select>
							{#if $createErrors.restart_policy}
								<p class="mt-1 text-sm text-red-300">{$createErrors.restart_policy[0]}</p>
							{/if}
						</div>

						<div class="flex justify-end gap-2">
							<Button type="button" variant="outline" onclick={() => goto(`/nodes/${nodeID}`)}>Cancel</Button>
							<Button type="submit" disabled={createSubmitting}>
								<Plus size={16} />
								Create server
							</Button>
						</div>
					</form>
				{/if}
			</CardContent>
		</Card>
	</div>
</main>
