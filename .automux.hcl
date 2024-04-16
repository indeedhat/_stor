session = "dotStor"
# config = "./tmux.conf"
# single_session = false 

window "Editor" {
    exec = "vim"
    focus = true

    # split {
    #     vertical = true
    #     exec = "cmd_to_run_in_split"
    #     size = 30
    #     vertical = true
    # }
}

window "Shell" {
    split {}
}
