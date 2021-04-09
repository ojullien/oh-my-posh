export POSH_THEME=::CONFIG::

function omp_preexec() {
  omp_start_time=$(::OMP:: --millis)
}

function omp_precmd() {
  omp_last_error=$?
  omp_elapsed=-1
  if [ $omp_start_time ]; then
    omp_now=$(::OMP:: --millis)
    omp_elapsed=$(($omp_now-$omp_start_time))
  fi
  eval "$(::OMP:: --config $POSH_THEME --error $omp_last_error --execution-time $omp_elapsed --eval --shell zsh)"
  unset omp_start_time
  unset omp_now
  unset omp_elapsed
  unset omp_last_error
}

function install_omp_hooks() {
  for s in "${preexec_functions[@]}"; do
    if [ "$s" = "omp_preexec" ]; then
      return
    fi
  done
  preexec_functions+=(omp_preexec)

  for s in "${precmd_functions[@]}"; do
    if [ "$s" = "omp_precmd" ]; then
      return
    fi
  done
  precmd_functions+=(omp_precmd)
}

if [ "$TERM" != "linux" ]; then
  install_omp_hooks
fi

function export_poshconfig() {
    [ $# -eq 0 ] && { echo "Usage: $0 \"filename\""; return; }
    format=$2
    if [ -z "$format" ]; then
      format="json"
    fi
    ::OMP:: --config $POSH_THEME --print-config --config-format $format > $1
}

function export_poshimage() {
    author=$1
    if [ -z "$author" ]; then
      ::OMP:: --config $POSH_THEME --export-png --shell shell
      return
    fi
    ::OMP:: --config $POSH_THEME --export-png --shell shell --author $author
}
