read -r -p "${1:-Are you sure? [Y/n]} " response
    case "$response" in 
        [nN][oO]|[nN]) 
            exit 1
            ;;
        *)
            exit 0
            ;;
    esac
