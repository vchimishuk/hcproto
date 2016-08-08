HipChat message parser
====================

Implementation notes:

 - Parse()/ParseJSON() implemented as a part of Parser interface, but not as a regular functions (which looks much more elegant to me), in the sake of two reasons: let the user control over network via HTTPGetter, and more convenient module testing.

Examples:

    c := &http.Client{
        // Tuning this timeout value we can limit maximum
        // message parsing time.
        Timeout: time.Second * 2,
    }
    p := NewParser(c)
    json, err := p.ParseJSON("Hi, @username!")
    if err != nil {
        panic("message parsing failed: " + err.Error())
    }
    fmt.Printf("%v\n", json)

P. S.

"How do mentions work" link in the document is broken.
