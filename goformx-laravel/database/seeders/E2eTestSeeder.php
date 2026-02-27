<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\Hash;

class E2eTestSeeder extends Seeder
{
    public function run(): void
    {
        User::updateOrCreate(
            ['email' => 'e2e@goformx.test'],
            [
                'name' => 'E2E Test User',
                'email_verified_at' => now(),
                'password' => Hash::make('E2eTestPass!2026'),
            ],
        );
    }
}
