local host_root = vim.fs.root(0, { ".git", "go.mod" }) or vim.fn.getcwd()
local proxy_bin = host_root .. "/remote-lsp-proxy"
local remote_root = "/workspaces/devcontainer.lsp"

vim.lsp.config("gopls", {
  cmd = {
    proxy_bin,
    "--local-workspace", host_root,
    "--remote-workspace", remote_root,
    "--remote-command", "gopls",
    "--remote-args", "serve",
  },
  root_dir = host_root,
  filetypes = { "go", "gomod", "gowork" },
  settings = {
    gopls = {
      semanticTokens = true,
    },
  },
})

vim.lsp.enable("gopls")
