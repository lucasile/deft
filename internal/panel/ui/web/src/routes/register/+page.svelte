<script lang="ts">
	import { auth } from '$lib/api/client';
	import { navigating } from '$app/stores';
	import { goto } from '$app/navigation';

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);

	const handleRegister = async (event: SubmitEvent) => {
		event.preventDefault();
		error = null;
		const response = await auth.register(username, password);

		if (response.ok) {
			const loginSuccess = await auth.login(username, password);
			if (loginSuccess) {
				goto('/');
			} else {
				error = 'Registration succeeded, but login failed. Please try logging in manually.';
			}
		} else {
			error = `Registration failed: ${await response.text()}`;
		}
	};
</script>

<h1>Register</h1>

<form onsubmit={handleRegister}>
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
		{$navigating ? 'Registering...' : 'Register'}
	</button>
</form>

<p>
	Already have an account? <a href="/login">Login</a>
</p>
