<!--
  GettingStarted.vue — Documentation / onboarding page.
  Explains what OpenBSR is and how to use it.
-->
<template>
  <div>
    <h2><i class="fas fa-cube"></i> What is OpenBSR?</h2>
    <p class="lead">
      OpenBSR is a <strong>self-hosted, open-source alternative</strong> to the
      <a href="https://buf.build" target="_blank">Buf Schema Registry (BSR)</a>.
      Host your own private Protobuf module registry that the
      <code>buf</code> CLI can push to and pull from.
    </p>

    <div class="row mb-4">
      <div class="col-md-4">
        <div class="card h-100">
          <div class="card-body text-center">
            <i class="fas fa-upload fa-2x text-primary mb-2"></i>
            <h5>Push Modules</h5>
            <p class="text-muted">Push <code>.proto</code> modules with <code>buf push</code> or the API.</p>
          </div>
        </div>
      </div>
      <div class="col-md-4">
        <div class="card h-100">
          <div class="card-body text-center">
            <i class="fas fa-download fa-2x text-success mb-2"></i>
            <h5>Pull Dependencies</h5>
            <p class="text-muted">Use modules as dependencies with <code>buf dep update</code>.</p>
          </div>
        </div>
      </div>
      <div class="col-md-4">
        <div class="card h-100">
          <div class="card-body text-center">
            <i class="fas fa-lock fa-2x text-warning mb-2"></i>
            <h5>Access Control</h5>
            <p class="text-muted">Public and private modules, organizations, role-based permissions.</p>
          </div>
        </div>
      </div>
    </div>

    <hr />
    <h3><i class="fas fa-rocket"></i> Quick Start</h3>

    <div class="card mb-4">
      <div class="card-header"><strong>Step 1:</strong> Create an Account</div>
      <div class="card-body">
        <p>
          <router-link to="/login">Register or log in</router-link>, then go to
          <router-link to="/tokens"><i class="fas fa-key"></i> Tokens</router-link>
          to create an API token for the <code>buf</code> CLI.
        </p>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header"><strong>Step 2:</strong> Create a Module</div>
      <div class="card-body">
        <p>Use the <router-link to="/create-module"><i class="fas fa-plus"></i> Create Module</router-link> page.</p>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header"><strong>Step 3:</strong> Configure <code>buf.yaml</code></div>
      <div class="card-body">
        <pre class="bg-dark text-light p-3 rounded"><code>version: v2
modules:
  - path: proto
    name: &lt;this-registry&gt;/&lt;your-username&gt;/mymodule</code></pre>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header"><strong>Step 4:</strong> Push</div>
      <div class="card-body">
        <pre class="bg-dark text-light p-3 rounded"><code>export BUF_TOKEN=&lt;your-api-token&gt;
buf push</code></pre>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header"><strong>Step 5:</strong> Use as a Dependency</div>
      <div class="card-body">
        <pre class="bg-dark text-light p-3 rounded"><code>deps:
  - &lt;this-registry&gt;/otheruser/theirmodule

# Then:
export BUF_TOKEN=&lt;your-api-token&gt;
buf dep update</code></pre>
      </div>
    </div>

    <hr />
    <h3><i class="fas fa-lock"></i> Permissions</h3>
    <table class="table table-sm">
      <thead><tr><th>Action</th><th>Who can do it</th></tr></thead>
      <tbody>
        <tr><td>View public modules</td><td>Anyone</td></tr>
        <tr><td>View private modules</td><td>Module owner or org member</td></tr>
        <tr><td>Create user-owned module</td><td>The user themselves</td></tr>
        <tr><td>Create org-owned module</td><td>Org admins only</td></tr>
        <tr><td>Push to user-owned module</td><td>The user themselves</td></tr>
        <tr><td>Push to org-owned module</td><td>Any org member</td></tr>
        <tr><td>Manage labels</td><td>Module owner or org admin</td></tr>
        <tr><td>Add/remove org members</td><td>Org admins only</td></tr>
      </tbody>
    </table>
  </div>
</template>
