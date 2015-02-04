#!/usr/bin/env python

def get_user_password(sockfile):
  """Given the path of a socket file, returns a tuple (user, password)."""
  return ("root", file('/etc/mysql/root.pw').read().strip())
