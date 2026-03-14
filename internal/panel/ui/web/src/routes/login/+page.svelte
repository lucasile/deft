<script lang="ts">
	import { auth } from '$lib/api/client';
	import { navigating } from '$app/stores';

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);

	const handleLogin = async (event: SubmitEvent) => {
		event.preventDefault();
		error = null;
		const success = await auth.login(username, password);
		if (!success) {
			error = 'Invalid username or password.';
		}
	};
</script>

<h1>Login</h1>

<form onsubmit={handleLogin}>
	<div>
		<label for="username">Username</label>
		<input id="username" type="text" bind:value={username} />
	</div>
	<div>
		<label for="password">Password</label>
		<input id="password" type="password" bind:value={password} />
	</div>

	{#if error}
		<p style="color: red;">{error}</p>
	{/if}

	<button type="submit" disabled={$navigating}>
		{$navigating ? 'Logging in...' : 'Log In'}
	</button>
</form>

<p>
	Don't have an account? <a href="/register">Register</a>
</p>
