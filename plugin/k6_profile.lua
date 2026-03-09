local k6p = require("k6_profile")

vim.api.nvim_create_user_command("K6ProfileLoad", function(opts)
  k6p.load(opts.args)
end, { nargs = 1, complete = "file" })

vim.api.nvim_create_user_command("K6ProfileApply", function()
  k6p.apply(0)
end, {})

vim.api.nvim_create_user_command("K6ProfileClear", function()
  k6p.clear(0)
end, {})

vim.api.nvim_create_autocmd({ "BufEnter", "BufWinEnter" }, {
  callback = function(args)
    if next(k6p.state.by_file) ~= nil then
      k6p.apply(args.buf)
    end
  end,
})
