ace.define("ace/snippets/bosun",["require","exports","module"], function(require, exports, module) {

exports.snippets = [
    /* Sections */
    {
        name: "section.alert_minimal",
        tabTrigger: "section.alert_minimal",
        content: 
"alert ${1:alertname} {\n\
    warn = ${2:numberSetExpression}\n\
}\n"
    },
    {
        name: "section.template_minimal",
        tabTrigger: "section.template_minimal",
        content: 
"template ${1:templatename} {\n\
    subject =\n\
}\n"
    },


    /* Combined Sections */
    {
        name: "section.alert_template_minimal",
        tabTrigger: "section.alert_template_minimal",
        content: 
"template ${1:name} {\n\
    subject = `${3:subject}`\n\
    body = `${4:body}`\n\
}\n\
\n\
alert ${1:name} {\n\
    template = ${1:name}\n\
    warn = ${2:numberSet}\n\
}\n"
    },

    /* Reduction Funcs */
    {
        name: "expr.avg",
        tabTrigger: "avg",
        content: "avg(${1:${SELECTED_TEXT:seriesSet}})"
    },

    /* Elastic Funcs */
    {
        name: "expr.escount",
        tabTrigger: "escount",
        content: "escount(${1:indexer}, \"${2:keysCSV}\", ${3:filter}, \"${4:bucketDuration}\", \"${5:startDuration}\", \"${6:endDuration}\")"
    },
    {
        name: "expr.esstat",
        tabTrigger: "esstat",
        content: "esstat(${1:indexer}, \"${2:keysCSV}\", ${3:filter}, \"${4:field}\", \"${5:reductFunc}\", \"${6:bucketDuration}\", \"${7:startDuration}\", \"${8:endDuration}\")"
    }
]

exports.scope = "bosun";

});
