<?php

test('sitemap returns 200 and application/xml', function () {
    config(['app.url' => 'https://example.com']);

    $response = $this->get(route('sitemap'));

    $response->assertOk();
    $response->assertHeader('Content-Type', 'application/xml');
});

test('sitemap contains home and demo URLs', function () {
    $appUrl = 'https://example.com';
    config(['app.url' => $appUrl]);

    $response = $this->get(route('sitemap'));

    $response->assertOk();
    $body = $response->getContent();
    expect($body)->toContain('<loc>'.$appUrl.'/</loc>');
    expect($body)->toContain('<loc>'.$appUrl.'/demo</loc>');
    expect($body)->toContain('<urlset');
    expect($body)->toContain('</urlset>');
});

test('robots.txt returns 200 and Sitemap directive', function () {
    $appUrl = 'https://example.com';
    config(['app.url' => $appUrl]);

    $response = $this->get(route('robots'));

    $response->assertOk();
    expect($response->headers->get('Content-Type'))->toStartWith('text/plain');
    expect($response->getContent())->toContain('User-agent: *');
    expect($response->getContent())->toContain('Sitemap: '.$appUrl.'/sitemap.xml');
});
