<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\Hash;

class E2eTestSeeder extends Seeder
{
    public function run(): void
    {
        User::factory()->create([
            'name' => 'E2E Test User',
            'email' => 'e2e@goformx.test',
            'email_verified_at' => now(),
            'password' => Hash::make('E2eTestPass!2026'),
        ]);
    }
}
