# exceptions: try/except/except*/else/finally, raise from
try:
    work()
except ValueError:
    handle()
except (KeyError, IndexError) as exc:
    handle(exc)
except:
    handle_any()
else:
    cleanup()
finally:
    finish()


try:
    risky()
except* TypeError:
    pass
except* (ValueError, KeyError) as group:
    pass
else:
    pass
finally:
    pass


raise
raise ValueError
raise ValueError("message")
raise ValueError("message") from cause
