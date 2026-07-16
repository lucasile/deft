<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Clock3, LogOut, Plus } from '@lucide/svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { auth, panel, type Node, type Recipe, type RecipeInput } from '$lib/api/client';
	import { backOrGoto } from '$lib/navigation';

	type RecipeValue = string | number | boolean;

	let nodes = $state<Node[]>([]);
	let recipes = $state<Recipe[]>([]);
	let recipeValues = $state<Record<string, RecipeValue>>({});
	let selectedRecipeID = $state('');
	let loading = $state(true);
	let createSubmitting = $state(false);
	let createMessage = $state('');
	let error = $state<string | null>(null);

	const nodeID = $derived(page.params.nodeID);
	const selectedNode = $derived(nodes.find((node) => node.id === nodeID));
	const selectedRecipe = $derived(recipes.find((recipe) => recipe.id === selectedRecipeID));

	onMount(() => {
		void loadPage();
	});

	const loadPage = async () => {
		loading = true;
		error = null;
		try {
			const [nodeList, recipeList] = await Promise.all([panel.nodes(), panel.recipes()]);
			nodes = nodeList;
			recipes = recipeList.filter((recipe) => recipe.enabled);
			if (recipes.length > 0 && !selectedRecipeID) {
				selectedRecipeID = recipes[0].id;
				resetRecipeValues(recipes[0]);
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

	const resetRecipeValues = (recipe: Recipe) => {
		const nextValues: Record<string, RecipeValue> = {};
		for (const input of recipe.inputs) {
			if (input.default !== undefined) {
				nextValues[input.key] = input.default;
			} else {
				nextValues[input.key] = input.type === 'number' ? 0 : '';
			}
		}
		recipeValues = nextValues;
	};

	const selectRecipe = (recipeID: string) => {
		selectedRecipeID = recipeID;
		const recipe = recipes.find((item) => item.id === recipeID);
		if (recipe) {
			resetRecipeValues(recipe);
		}
	};

	const setRecipeValue = (input: RecipeInput, rawValue: string) => {
		let value: RecipeValue = rawValue;
		if (input.type === 'number') {
			value = rawValue === '' ? '' : Number(rawValue);
		}
		recipeValues = { ...recipeValues, [input.key]: value };
	};

	const createServer = async () => {
		if (!nodeID || !selectedRecipe || createSubmitting) return;

		createSubmitting = true;
		createMessage = 'Creating server...';
		error = null;
		try {
			const response = await panel.createServerFromRecipe(nodeID, selectedRecipe.id, recipeValues);
			createMessage = 'Server created. Opening server page...';
			await goto(`/servers/${response.server_id || response.command_id}`);
		} catch (err) {
			error = cleanError(err);
			createMessage = '';
		} finally {
			createSubmitting = false;
		}
	};

	const logout = async () => {
		await auth.logout();
		goto('/login');
	};

	const cleanError = (err: unknown) => {
		return err instanceof Error ? err.message.trim() : 'Request failed';
	};
</script>

<svelte:head>
	<title>Create Server - Deft Panel</title>
</svelte:head>

<main class="min-h-screen bg-zinc-950 text-zinc-100">
	<header class="border-b border-zinc-800 bg-zinc-950/95">
		<div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
			<div class="min-w-0">
				<Button type="button" variant="ghost" class="mb-2 px-0 text-zinc-400 hover:text-white" onclick={() => backOrGoto(`/nodes/${nodeID}`)}>
					<ArrowLeft size={16} />
					Back
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

	<div class="mx-auto max-w-3xl px-4 py-6 sm:px-6 lg:px-8">
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
				{#if createMessage}
					<div class="mb-4 rounded-md border border-emerald-900/60 bg-emerald-950/50 px-3 py-2 text-sm text-emerald-200">
						{createMessage}
					</div>
				{/if}

				{#if loading}
					<p class="text-sm text-zinc-400">Loading agent...</p>
				{:else if !selectedNode}
					<p class="text-sm text-zinc-400">Agent not found.</p>
				{:else if !selectedNode.connected}
					<p class="text-sm text-zinc-400">Start the agent before creating servers.</p>
				{:else if recipes.length === 0}
					<p class="text-sm text-zinc-400">No recipes are available.</p>
				{:else if selectedRecipe}
					<form
						class="space-y-5"
						onsubmit={(event) => {
							event.preventDefault();
							void createServer();
						}}
					>
						<div>
							<Label for="server-recipe">Recipe</Label>
							<select
								id="server-recipe"
								value={selectedRecipeID}
								class="mt-2 h-10 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 text-sm text-zinc-100 outline-none focus:border-zinc-500"
								disabled={createSubmitting}
								onchange={(event) => selectRecipe(event.currentTarget.value)}
							>
								{#each recipes as recipe (recipe.id)}
									<option value={recipe.id}>{recipe.name}</option>
								{/each}
							</select>
							<p class="mt-2 text-sm text-zinc-400">{selectedRecipe.description}</p>
						</div>

						<div class="grid gap-4 md:grid-cols-2">
							{#each selectedRecipe.inputs as input (input.key)}
								<div>
									<div class="flex items-center gap-2">
										<Label for={`recipe-${input.key}`}>{input.label}</Label>
										{#if input.editable_by === 'admin'}
											<Badge variant="default">admin</Badge>
										{/if}
									</div>
									{#if input.type === 'select'}
										<select
											id={`recipe-${input.key}`}
											value={String(recipeValues[input.key] ?? '')}
											class="mt-2 h-10 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 text-sm text-zinc-100 outline-none focus:border-zinc-500"
											required={input.required}
											disabled={createSubmitting}
											onchange={(event) => setRecipeValue(input, event.currentTarget.value)}
										>
											{#each input.options || [] as option (option)}
												<option value={option}>{option}</option>
											{/each}
										</select>
									{:else}
										<Input
											id={`recipe-${input.key}`}
											type={input.type === 'number' ? 'number' : 'text'}
											value={String(recipeValues[input.key] ?? '')}
											min={input.min}
											max={input.max}
											required={input.required}
											autocomplete="off"
											disabled={createSubmitting}
											oninput={(event) => setRecipeValue(input, event.currentTarget.value)}
										/>
									{/if}
								</div>
							{/each}
						</div>

						<div class="flex justify-end gap-2">
							<Button type="button" variant="outline" disabled={createSubmitting} onclick={() => backOrGoto(`/nodes/${nodeID}`)}>Cancel</Button>
							<Button type="submit" disabled={createSubmitting}>
								<Plus size={16} />
								{createSubmitting ? 'Creating...' : 'Create server'}
							</Button>
						</div>
					</form>
				{/if}
			</CardContent>
		</Card>
	</div>
</main>
