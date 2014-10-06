/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="d3.d.ts" />
/// <reference path="underscore.d.ts" />
var bosunApp = angular.module('bosunApp', [
    'ngRoute',
    'bosunControllers',
    'mgcrea.ngStrap',
    'ngSanitize',
]);
bosunApp.config(['$routeProvider', '$locationProvider', function ($routeProvider, $locationProvider) {
    $locationProvider.html5Mode(true);
    $routeProvider.when('/', {
        title: 'Dashboard',
        templateUrl: 'partials/dashboard.html',
        controller: 'DashboardCtrl'
    }).when('/items', {
        title: 'Items',
        templateUrl: 'partials/items.html',
        controller: 'ItemsCtrl'
    }).when('/expr', {
        title: 'Expression',
        templateUrl: 'partials/expr.html',
        controller: 'ExprCtrl'
    }).when('/graph', {
        title: 'Graph',
        templateUrl: 'partials/graph.html',
        controller: 'GraphCtrl'
    }).when('/host', {
        title: 'Host View',
        templateUrl: 'partials/host.html',
        controller: 'HostCtrl',
        reloadOnSearch: false
    }).when('/rule', {
        title: 'Rule',
        templateUrl: 'partials/rule.html',
        controller: 'RuleCtrl'
    }).when('/silence', {
        title: 'Silence',
        templateUrl: 'partials/silence.html',
        controller: 'SilenceCtrl'
    }).when('/config', {
        title: 'Configuration',
        templateUrl: 'partials/config.html',
        controller: 'ConfigCtrl'
    }).when('/action', {
        title: 'Action',
        templateUrl: 'partials/action.html',
        controller: 'ActionCtrl'
    }).when('/history', {
        title: 'Alert History',
        templateUrl: 'partials/history.html',
        controller: 'HistoryCtrl'
    }).when('/put', {
        title: 'Data Entry',
        templateUrl: 'partials/put.html',
        controller: 'PutCtrl'
    }).otherwise({
        redirectTo: '/'
    });
}]);
bosunApp.run(['$location', '$rootScope', function ($location, $rootScope) {
    $rootScope.$on('$routeChangeSuccess', function (event, current, previous) {
        $rootScope.title = current.$$route.title;
        $rootScope.shortlink = false;
    });
}]);
var bosunControllers = angular.module('bosunControllers', []);
bosunControllers.controller('BosunCtrl', ['$scope', '$route', '$http', '$q', '$rootScope', function ($scope, $route, $http, $q, $rootScope) {
    $scope.$on('$routeChangeSuccess', function (event, current, previous) {
        $scope.stop(true);
    });
    $scope.active = function (v) {
        if (!$route.current) {
            return null;
        }
        if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
            return { active: true };
        }
        return null;
    };
    $scope.json = function (v) {
        return JSON.stringify(v, null, '  ');
    };
    $scope.btoa = function (v) {
        return encodeURIComponent(btoa(v));
    };
    $scope.encode = function (v) {
        return encodeURIComponent(v);
    };
    $scope.req_from_m = function (m) {
        var r = new Request();
        var q = new Query();
        q.metric = m;
        r.queries.push(q);
        return r;
    };
    $scope.panelClass = function (status, prefix) {
        if (prefix === void 0) { prefix = "panel-"; }
        switch (status) {
            case "critical":
                return prefix + "danger";
            case "unknown":
                return prefix + "info";
            case "warning":
                return prefix + "warning";
            case "normal":
                return prefix + "success";
            case "error":
                return prefix + "danger";
            default:
                return prefix + "default";
        }
    };
    var scheduleFilter;
    $scope.refresh = function (filter) {
        var d = $q.defer();
        scheduleFilter = filter;
        $scope.animate();
        var p = $http.get('/api/alerts?filter=' + encodeURIComponent(filter || "")).success(function (data) {
            $scope.schedule = data;
            $scope.timeanddate = data.TimeAndDate;
            d.resolve();
        }).error(function (err) {
            d.reject(err);
        });
        p.finally($scope.stop);
        return d.promise;
    };
    var sz = 30;
    var orig = 700;
    var light = '#4ba2d9';
    var dark = '#1f5296';
    var med = '#356eb6';
    var mult = sz / orig;
    var bgrad = 25 * mult;
    var circles = [
        [150, 150, dark],
        [550, 150, dark],
        [150, 550, light],
        [550, 550, light],
    ];
    var svg = d3.select('#logo').append('svg').attr('height', sz).attr('width', sz);
    svg.selectAll('rect.bg').data([[0, light], [sz / 2, dark]]).enter().append('rect').attr('class', 'bg').attr('width', sz).attr('height', sz / 2).attr('rx', bgrad).attr('ry', bgrad).attr('fill', function (d) {
        return d[1];
    }).attr('y', function (d) {
        return d[0];
    });
    svg.selectAll('path.diamond').data([150, 550]).enter().append('path').attr('d', function (d) {
        var s = 'M ' + d * mult + ' ' + 150 * mult;
        var w = 200 * mult;
        s += ' l ' + w + ' ' + w;
        s += ' l ' + -w + ' ' + w;
        s += ' l ' + -w + ' ' + -w + ' Z';
        return s;
    }).attr('fill', med).attr('stroke', 'white').attr('stroke-width', 15 * mult);
    svg.selectAll('rect.white').data([150, 350, 550]).enter().append('rect').attr('class', 'white').attr('width', .5).attr('height', '100%').attr('fill', 'white').attr('x', function (d) {
        return d * mult;
    });
    svg.selectAll('circle').data(circles).enter().append('circle').attr('cx', function (d) {
        return d[0] * mult;
    }).attr('cy', function (d) {
        return d[1] * mult;
    }).attr('r', 62.5 * mult).attr('fill', function (d) {
        return d[2];
    }).attr('stroke', 'white').attr('stroke-width', 25 * mult);
    var transitionDuration = 750;
    var animateCount = 0;
    $scope.animate = function () {
        animateCount++;
        if (animateCount == 1) {
            doAnimate();
        }
    };
    function doAnimate() {
        if (!animateCount) {
            return;
        }
        d3.shuffle(circles);
        svg.selectAll('circle').data(circles, function (d, i) {
            return i;
        }).transition().duration(transitionDuration).attr('cx', function (d) {
            return d[0] * mult;
        }).attr('cy', function (d) {
            return d[1] * mult;
        }).attr('fill', function (d) {
            return d[2];
        });
        setTimeout(doAnimate, transitionDuration);
    }
    $scope.stop = function (all) {
        if (all === void 0) { all = false; }
        if (all) {
            animateCount = 0;
        }
        else if (animateCount > 0) {
            animateCount--;
        }
    };
    var short = $('#shortlink')[0];
    $scope.shorten = function () {
        $http.post('https://www.googleapis.com/urlshortener/v1/url', {
            longUrl: document.URL
        }).success(function (data) {
            if (data.id) {
                short.value = data.id;
                $rootScope.shortlink = true;
                setTimeout(function () {
                    short.setSelectionRange(0, data.id.length);
                });
            }
        });
    };
}]);
moment.defaultFormat = 'YYYY/MM/DD-HH:mm:ss';
moment.lang('en', {
    relativeTime: {
        future: "in %s",
        past: "%s-ago",
        s: "%ds",
        m: "%dm",
        mm: "%dm",
        h: "%dh",
        hh: "%dh",
        d: "%dd",
        dd: "%dd",
        M: "%dn",
        MM: "%dn",
        y: "%dy",
        yy: "%dy"
    }
});
function createCookie(name, value, days) {
    var expires;
    if (days) {
        var date = new Date();
        date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
        expires = "; expires=" + date.toGMTString();
    }
    else {
        expires = "";
    }
    document.cookie = escape(name) + "=" + escape(value) + expires + "; path=/";
}
function readCookie(name) {
    var nameEQ = escape(name) + "=";
    var ca = document.cookie.split(';');
    for (var i = 0; i < ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) === ' ')
            c = c.substring(1, c.length);
        if (c.indexOf(nameEQ) === 0)
            return unescape(c.substring(nameEQ.length, c.length));
    }
    return null;
}
function eraseCookie(name) {
    createCookie(name, "", -1);
}
// from: http://stackoverflow.com/a/15267754/864236
bosunApp.filter('reverse', function () {
    return function (items) {
        if (!angular.isArray(items)) {
            return [];
        }
        return items.slice().reverse();
    };
});
bosunControllers.controller('ActionCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    $scope.user = readCookie("action-user");
    $scope.type = search.type;
    if (!angular.isArray(search.key)) {
        $scope.keys = [search.key];
    }
    else {
        $scope.keys = search.key;
    }
    $scope.submit = function () {
        var data = {
            Type: $scope.type,
            User: $scope.user,
            Message: $scope.message,
            Keys: $scope.keys
        };
        createCookie("action-user", $scope.user, 1000);
        $http.post('/api/action', data).success(function (data) {
            $location.url('/');
        }).error(function (error) {
            alert(error);
        });
    };
}]);
bosunControllers.controller('ConfigCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    var current = search.config_text;
    var line_re = /test:(\d+)/;
    try {
        current = atob(current);
    }
    catch (e) {
        current = '';
    }
    if (!current) {
        var def = '';
        $http.get('/api/config').success(function (data) {
            def = data;
        }).finally(function () {
            $location.search('config_text', btoa(def));
        });
        return;
    }
    $scope.config_text = current;
    $scope.set = function () {
        $scope.result = null;
        $scope.line = null;
        $http.get('/api/config_test?config_text=' + encodeURIComponent($scope.config_text)).success(function (data) {
            if (data == "") {
                $scope.result = "Valid";
            }
            else {
                $scope.result = data;
                var m = data.match(line_re);
                if (angular.isArray(m) && (m.length > 1)) {
                    $scope.line = m[1];
                }
            }
        }).error(function (error) {
            $scope.error = error || 'Error';
        });
    };
    $scope.set();
}]);
bosunControllers.controller('DashboardCtrl', ['$scope', '$location', function ($scope, $location) {
    var search = $location.search();
    $scope.loading = 'Loading';
    $scope.error = '';
    $scope.filter = search.filter;
    if (!$scope.filter) {
        $scope.filter = readCookie("filter");
    }
    $location.search('filter', $scope.filter || null);
    $scope.refresh($scope.filter).then(function () {
        $scope.loading = '';
    }, function (err) {
        $scope.loading = '';
        $scope.error = 'Unable to fetch alerts: ' + err;
    });
    $scope.keydown = function ($event) {
        if ($event.keyCode == 13) {
            createCookie("filter", $scope.filter || "", 1000);
            $location.search('filter', $scope.filter || null);
        }
    };
}]);
bosunApp.directive('tsResults', function () {
    return {
        templateUrl: '/partials/results.html',
        link: function (scope, elem, attrs) {
            scope.isSeries = function (v) {
                return typeof (v) === 'object';
            };
        }
    };
});
bosunApp.directive('tsComputations', function () {
    return {
        scope: {
            computations: '=tsComputations',
            time: '=',
            header: '='
        },
        templateUrl: '/partials/computations.html',
        link: function (scope, elem, attrs) {
            if (scope.time) {
                var m = moment.utc(scope.time, timeFormat);
                scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
            }
            scope.btoa = function (v) {
                return encodeURIComponent(btoa(v));
            };
        }
    };
});
var timeFormat = 'YYYY/MM/DD-HH:mm:ss';
function fmtTime(v) {
    var m = moment(v).utc();
    return m.format(timeFormat) + ' (' + m.fromNow() + ')';
}
bosunApp.directive("tsTime", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsTime, function (v) {
                var text = fmtTime(v);
                if (attrs.noLink) {
                    elem.text(text);
                }
                else {
                    var el = document.createElement('a');
                    var m = moment(v).utc();
                    el.innerText = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, function (v, k) {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        }
    };
});
bosunApp.directive("tsSince", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsSince, function (v) {
                var m = moment(v).utc();
                elem.text(m.fromNow());
            });
        }
    };
});
bosunApp.directive("tooltip", function () {
    return {
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});
bosunApp.directive('tsLine', function () {
    return {
        link: function (scope, elem, attrs) {
            elem.linedtextarea();
            var parent = elem.parent();
            var linesDiv = parent;
            function lineHighlight(line) {
                var lineHeight = elem[0].scrollHeight / (elem[0].value.match(/\n/g).length + 1);
                var jump = (line - 1) * lineHeight;
                elem.scrollTop(jump);
                elem.scroll();
                parent.find('.lines div').eq(line - 1).addClass('lineerror');
            }
            function lineClear() {
                parent.find('.lineerror').removeClass('lineerror');
            }
            scope.$watch(attrs.tsLine, function (v) {
                lineClear();
                if (v) {
                    lineHighlight(v);
                }
            });
        }
    };
});
bosunApp.directive('tsTab', function () {
    return {
        link: function (scope, elem, attrs) {
            var ta = elem[0];
            elem.keydown(function (evt) {
                if (evt.ctrlKey) {
                    return;
                }
                switch (evt.keyCode) {
                    case 9:
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        ta.value = v.substr(0, start) + "\t" + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1;
                        return;
                    case 13:
                        if (ta.selectionStart != ta.selectionEnd) {
                            return;
                        }
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        var sub = v.substr(0, start);
                        var last = sub.lastIndexOf("\n") + 1;
                        for (var i = last; i < sub.length && /[ \t]/.test(sub[i]); i++)
                            ;
                        var ws = sub.substr(last, i - last);
                        ta.value = v.substr(0, start) + "\n" + ws + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1 + ws.length;
                }
            });
        }
    };
});
bosunApp.directive('tsTableSort', ['$timeout', function ($timeout) {
    return {
        link: function (scope, elem, attrs) {
            $timeout(function () {
                $(elem).tablesorter({
                    sortList: scope.$eval(attrs.tsTableSort)
                });
            });
        }
    };
}]);
bosunApp.directive('tsHistory', function () {
    return {
        scope: {
            computations: '=tsComputations',
            time: '=',
            header: '='
        },
        templateUrl: '/partials/history.html',
        link: function (scope, elem, attrs) {
            if (scope.time) {
                var m = moment.utc(scope.time, timeFormat);
                scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
            }
            scope.btoa = function (v) {
                return encodeURIComponent(btoa(v));
            };
        }
    };
});
bosunApp.directive('tsTimeLine', function () {
    var format = d3.time.format.utc("%Y-%m-%dT%X");
    var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
    function parseDate(s) {
        return s.toDate();
    }
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 250
    };
    return {
        link: function (scope, elem, attrs) {
            scope.shown = {};
            scope.collapse = function (i) {
                scope.shown[i] = !scope.shown[i];
            };
            scope.$watch('alert_history', update);
            function update(history) {
                if (!history) {
                    return;
                }
                var entries = d3.entries(history);
                if (!entries.length) {
                    return;
                }
                entries.sort(function (a, b) {
                    return a.key.localeCompare(b.key);
                });
                scope.entries = entries;
                var values = entries.map(function (v) {
                    return v.value;
                });
                var keys = entries.map(function (v) {
                    return v.key;
                });
                var barheight = 500 / values.length;
                barheight = Math.min(barheight, 45);
                barheight = Math.max(barheight, 15);
                var svgHeight = values.length * barheight + margin.top + margin.bottom;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth = elem.width();
                var width = svgWidth - margin.left - margin.right;
                var xScale = d3.time.scale.utc().range([0, width]);
                var xAxis = d3.svg.axis().scale(xScale).orient('bottom');
                elem.empty();
                var svg = d3.select(elem[0]).append('svg').attr('width', svgWidth).attr('height', svgHeight).append('g').attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                svg.append('g').attr('class', 'x axis tl-axis').attr('transform', 'translate(0,' + height + ')');
                xScale.domain([
                    d3.min(values, function (d) {
                        return d3.min(d.History, function (c) {
                            return c.Time;
                        });
                    }),
                    d3.max(values, function (d) {
                        return d3.max(d.History, function (c) {
                            return c.EndTime;
                        });
                    }),
                ]);
                var legend = d3.select(elem[0]).append('div').attr('class', 'tl-legend');
                var time_legend = legend.append('div').text(tsdbFormat(new Date()));
                var alert_legend = legend.append('div').text('Alert');
                svg.select('.x.axis').transition().call(xAxis);
                var chart = svg.append('g');
                angular.forEach(entries, function (entry, i) {
                    chart.selectAll('.bars').data(entry.value.History).enter().append('rect').attr('class', function (d) {
                        return 'tl-' + d.Status;
                    }).attr('x', function (d) {
                        return xScale(parseDate(d.Time));
                    }).attr('y', i * barheight).attr('height', barheight).attr('width', function (d) {
                        return xScale(parseDate(d.EndTime)) - xScale(parseDate(d.Time));
                    }).on('mousemove.x', mousemove_x).on('mousemove.y', function (d) {
                        alert_legend.text(entry.key);
                    }).on('click', function (d, j) {
                        var id = 'panel' + i + '-' + j;
                        scope.shown['group' + i] = true;
                        scope.shown[id] = true;
                        scope.$apply();
                        setTimeout(function () {
                            var e = $("#" + id);
                            if (!e) {
                                console.log('no', id, e);
                                return;
                            }
                            $('html, body').scrollTop(e.offset().top);
                        });
                    });
                });
                chart.selectAll('.labels').data(keys).enter().append('text').attr('text-anchor', 'end').attr('x', 0).attr('dx', '-.5em').attr('dy', '.25em').attr('y', function (d, i) {
                    return (i + .5) * barheight;
                }).text(function (d) {
                    return d;
                });
                chart.selectAll('.sep').data(values).enter().append('rect').attr('y', function (d, i) {
                    return (i + 1) * barheight;
                }).attr('height', 1).attr('x', 0).attr('width', width).on('mousemove.x', mousemove_x);
                function mousemove_x() {
                    var x = xScale.invert(d3.mouse(this)[0]);
                    time_legend.text(tsdbFormat(x));
                }
            }
            ;
        }
    };
});
var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];
function nfmt(s, mult, suffix, opts) {
    opts = opts || {};
    var n = parseFloat(s);
    if (isNaN(n) && typeof s === 'string') {
        return s;
    }
    if (opts.round)
        n = Math.round(n);
    if (!n)
        return suffix ? '0 ' + suffix : '0';
    if (isNaN(n) || !isFinite(n))
        return '-';
    var a = Math.abs(n);
    if (a >= 1) {
        var number = Math.floor(Math.log(a) / Math.log(mult));
        a /= Math.pow(mult, Math.floor(number));
        if (fmtUnits[number]) {
            suffix = fmtUnits[number] + suffix;
        }
    }
    var r = a.toFixed(5);
    if (a < 1e-5) {
        r = a.toString();
    }
    var neg = n < 0 ? '-' : '';
    return neg + (+r) + suffix;
}
bosunApp.filter('nfmt', function () {
    return function (s) {
        return nfmt(s, 1000, '', {});
    };
});
bosunApp.filter('bytes', function () {
    return function (s) {
        return nfmt(s, 1024, 'B', { round: true });
    };
});
bosunApp.filter('bits', function () {
    return function (s) {
        return nfmt(s, 1024, 'b', { round: true });
    };
});
bosunApp.directive('tsGraph', ['$window', 'nfmtFilter', function ($window, fmtfilter) {
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 80
    };
    return {
        scope: {
            data: '=',
            height: '=',
            generator: '=',
            brushStart: '=bstart',
            brushEnd: '=bend',
            enableBrush: '@'
        },
        link: function (scope, elem, attrs) {
            var svgHeight = +scope.height || 150;
            var height = svgHeight - margin.top - margin.bottom;
            var svgWidth;
            var width;
            var yScale = d3.scale.linear().range([height, 0]);
            var xScale = d3.time.scale.utc();
            var xAxis = d3.svg.axis().orient('bottom');
            var yAxis = d3.svg.axis().scale(yScale).orient('left').ticks(Math.min(10, height / 20)).tickFormat(fmtfilter);
            var line;
            switch (scope.generator) {
                case 'area':
                    line = d3.svg.area();
                    break;
                default:
                    line = d3.svg.line();
            }
            var brush = d3.svg.brush().x(xScale).on('brush', brushed);
            line.y(function (d) {
                return yScale(d[1]);
            });
            line.x(function (d) {
                return xScale(d[0] * 1000);
            });
            var top = d3.select(elem[0]).append('svg').attr('height', svgHeight).attr('width', '100%');
            var svg = top.append('g').attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
            var defs = svg.append('defs').append('clipPath').attr('id', 'clip').append('rect').attr('height', height);
            var chart = svg.append('g').attr('pointer-events', 'all').attr('clip-path', 'url(#clip)');
            svg.append('g').attr('class', 'x axis').attr('transform', 'translate(0,' + height + ')');
            svg.append('g').attr('class', 'y axis');
            var paths = chart.append('g');
            chart.append('g').attr('class', 'x brush');
            top.append('rect').style('opacity', 0).attr('x', 0).attr('y', 0).attr('height', height).attr('width', margin.left).style('cursor', 'pointer').on('click', yaxisToggle);
            var legendTop = d3.select(elem[0]).append('div');
            var xloc = legendTop.append('div');
            xloc.style('float', 'left');
            var brushText = legendTop.append('div');
            brushText.style('float', 'right');
            var legend = d3.select(elem[0]).append('div');
            legend.style('clear', 'both');
            var color = d3.scale.ordinal().range([
                '#e41a1c',
                '#377eb8',
                '#4daf4a',
                '#984ea3',
                '#ff7f00',
                '#a65628',
                '#f781bf',
                '#999999',
            ]);
            var mousex = 0;
            var mousey = 0;
            var oldx = 0;
            var hover = svg.append('g').attr('class', 'hover').style('pointer-events', 'none').style('display', 'none');
            var hoverPoint = hover.append('svg:circle').attr('r', 5);
            var hoverRect = hover.append('svg:rect').attr('fill', 'white');
            var hoverText = hover.append('svg:text').style('font-size', '12px');
            var focus = svg.append('g').attr('class', 'focus').style('pointer-events', 'none');
            focus.append('line');
            function mousemove() {
                var pt = d3.mouse(this);
                mousex = pt[0];
                mousey = pt[1];
                if (scope.data) {
                    drawLegend();
                }
            }
            var yaxisZero = false;
            function yaxisToggle() {
                yaxisZero = !yaxisZero;
                draw();
            }
            var drawLegend = _.throttle(function () {
                var names = legend.selectAll('.series').data(scope.data, function (d) {
                    return d.Name;
                });
                names.enter().append('div').attr('class', 'series');
                names.exit().remove();
                var xi = xScale.invert(mousex);
                xloc.text('Time: ' + fmtTime(xi));
                var t = xi.getTime() / 1000;
                var minDist = width + height;
                var minName, minColor;
                var minX, minY;
                names.each(function (d) {
                    var idx = bisect(d.Data, t);
                    if (idx >= d.Data.length) {
                        idx = d.Data.length - 1;
                    }
                    var e = d3.select(this);
                    var pt = d.Data[idx];
                    if (pt) {
                        e.attr('title', pt[1]);
                        e.text(d.Name + ': ' + fmtfilter(pt[1]));
                        var ptx = xScale(pt[0] * 1000);
                        var pty = yScale(pt[1]);
                        var ptd = Math.sqrt(Math.pow(ptx - mousex, 2) + Math.pow(pty - mousey, 2));
                        if (ptd < minDist) {
                            minDist = ptd;
                            minX = ptx;
                            minY = pty;
                            minName = d.Name + ': ' + pt[1];
                            minColor = color(d.Name);
                        }
                    }
                }).style('color', function (d) {
                    return color(d.Name);
                });
                hover.attr('transform', 'translate(' + minX + ',' + minY + ')').style('display', 'block');
                hoverPoint.style('fill', minColor);
                hoverText.text(minName).style('fill', minColor);
                var isRight = minX > width / 2;
                var isBottom = minY > height / 2;
                hoverText.attr('x', isRight ? -5 : 5).attr('y', isBottom ? -8 : 15).attr('text-anchor', isRight ? 'end' : 'start');
                var node = hoverText.node();
                var bb = node.getBBox();
                hoverRect.attr('x', bb.x - 1).attr('y', bb.y - 1).attr('height', bb.height + 2).attr('width', bb.width + 2);
                var x = mousex;
                if (x > width) {
                    x = 0;
                }
                focus.select('line').attr('x1', x).attr('x2', x).attr('y1', 0).attr('y2', height);
                if (extentStart) {
                    var s = extentStart;
                    if (extentEnd != extentStart) {
                        s += ' - ' + extentEnd;
                    }
                    brushText.text(s);
                }
            }, 50);
            scope.$watch('data', update);
            var w = angular.element($window);
            scope.$watch(function () {
                return w.width();
            }, resize, true);
            w.bind('resize', function () {
                scope.$apply();
            });
            function resize() {
                svgWidth = elem.width();
                if (svgWidth <= 0) {
                    return;
                }
                width = svgWidth - margin.left - margin.right;
                xScale.range([0, width]);
                xAxis.scale(xScale);
                if (!mousex) {
                    mousex = width + 1;
                }
                svg.attr('width', svgWidth);
                defs.attr('width', width);
                xAxis.ticks(width / 60);
                draw();
            }
            var oldx = 0;
            var bisect = d3.bisector(function (d) {
                return d[0];
            }).left;
            function update(v) {
                if (!angular.isArray(v) || v.length == 0) {
                    return;
                }
                resize();
            }
            function draw() {
                if (!scope.data) {
                    return;
                }
                var xdomain = [
                    d3.min(scope.data, function (d) {
                        return d3.min(d.Data, function (c) {
                            return c[0];
                        });
                    }) * 1000,
                    d3.max(scope.data, function (d) {
                        return d3.max(d.Data, function (c) {
                            return c[0];
                        });
                    }) * 1000,
                ];
                if (!oldx) {
                    oldx = xdomain[1];
                }
                xScale.domain(xdomain);
                var ymin = d3.min(scope.data, function (d) {
                    return d3.min(d.Data, function (c) {
                        return c[1];
                    });
                });
                var ymax = d3.max(scope.data, function (d) {
                    return d3.max(d.Data, function (c) {
                        return c[1];
                    });
                });
                var diff = (ymax - ymin) / 50;
                if (!diff) {
                    diff = 1;
                }
                ymin -= diff;
                ymax += diff;
                if (yaxisZero) {
                    if (ymin > 0) {
                        ymin = 0;
                    }
                    else if (ymax < 0) {
                        ymax = 0;
                    }
                }
                var ydomain = [ymin, ymax];
                yScale.domain(ydomain);
                if (scope.generator == 'area') {
                    line.y0(yScale(0));
                }
                svg.select('.x.axis').transition().call(xAxis);
                svg.select('.y.axis').transition().call(yAxis);
                var queries = paths.selectAll('.line').data(scope.data, function (d) {
                    return d.Name;
                });
                switch (scope.generator) {
                    case 'area':
                        queries.enter().append('path').attr('stroke', function (d) {
                            return color(d.Name);
                        }).attr('class', 'line').style('fill', function (d) {
                            return color(d.Name);
                        });
                        break;
                    default:
                        queries.enter().append('path').attr('stroke', function (d) {
                            return color(d.Name);
                        }).attr('class', 'line');
                }
                queries.exit().remove();
                queries.attr('d', function (d) {
                    return line(d.Data);
                }).attr('transform', null).transition().ease('linear').attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
                chart.select('.x.brush').call(brush).selectAll('rect').attr('height', height).on('mousemove', mousemove);
                chart.select('.x.brush .extent').style('stroke', '#fff').style('fill-opacity', '.125').style('shape-rendering', 'crispEdges');
                oldx = xdomain[1];
                drawLegend();
            }
            ;
            var extentStart;
            var extentEnd;
            function brushed() {
                var extent = brush.extent();
                extentStart = datefmt(extent[0]);
                extentEnd = datefmt(extent[1]);
                drawLegend();
                if (scope.enableBrush) {
                    scope.brushStart = extentStart;
                    scope.brushEnd = extentEnd;
                    scope.$apply();
                }
            }
            var mfmt = 'YYYY/MM/DD-HH:mm:ss';
            function datefmt(d) {
                return moment(d).utc().format(mfmt);
            }
        }
    };
}]);
bosunControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    var current;
    try {
        current = atob(search.expr);
    }
    catch (e) {
        current = '';
    }
    if (!current) {
        $location.search('expr', btoa('avg(q("avg:rate:os.cpu{host=*bosun*}", "5m", "")) > 80'));
        return;
    }
    $scope.date = search.date || '';
    $scope.time = search.time || '';
    $scope.expr = current;
    $scope.running = current;
    $scope.tab = 'results';
    $scope.animate();
    $http.get('/api/expr?q=' + encodeURIComponent(current) + '&date=' + encodeURIComponent($scope.date) + '&time=' + encodeURIComponent($scope.time)).success(function (data) {
        $scope.result = data.Results;
        $scope.queries = data.Queries;
        $scope.result_type = data.Type;
        if (data.Type == 'series') {
            $scope.svg_url = '/api/egraph/' + btoa(current) + '.svg?now=' + Math.floor(Date.now() / 1000);
            $scope.graph = toChart(data.Results);
        }
        $scope.running = '';
    }).error(function (error) {
        $scope.error = error;
        $scope.running = '';
    }).finally(function () {
        $scope.stop();
    });
    $scope.set = function () {
        $location.search('expr', btoa($scope.expr));
        $location.search('date', $scope.date || null);
        $location.search('time', $scope.time || null);
        $route.reload();
    };
    function toChart(res) {
        var graph = [];
        angular.forEach(res, function (d, idx) {
            var data = [];
            angular.forEach(d.Value, function (val, ts) {
                data.push([+ts, val]);
            });
            if (data.length == 0) {
                return;
            }
            var name = '{';
            angular.forEach(d.Group, function (tagv, tagk) {
                if (name.length > 1) {
                    name += ',';
                }
                name += tagk + '=' + tagv;
            });
            name += '}';
            var series = {
                Data: data,
                Name: name
            };
            graph[idx] = series;
        });
        return graph;
    }
    $scope.keydown = function ($event) {
        if ($event.keyCode == 13) {
            $scope.set();
        }
    };
}]);
var TagSet = (function () {
    function TagSet() {
    }
    return TagSet;
})();
var TagV = (function () {
    function TagV() {
    }
    return TagV;
})();
var RateOptions = (function () {
    function RateOptions() {
    }
    return RateOptions;
})();
var Query = (function () {
    function Query(q) {
        this.aggregator = q && q.aggregator || 'sum';
        this.metric = q && q.metric || '';
        this.rate = q && q.rate || false;
        this.rateOptions = q && q.rateOptions || new RateOptions;
        if (q && !q.derivative) {
            // back compute derivative from q
            if (!this.rate) {
                this.derivative = 'gauge';
            }
            else if (this.rateOptions.counter) {
                this.derivative = 'counter';
            }
            else {
                this.derivative = 'rate';
            }
        }
        else {
            this.derivative = q && q.derivative || 'auto';
        }
        this.ds = q && q.ds || '';
        this.dstime = q && q.dstime || '';
        this.tags = q && q.tags || new TagSet;
        this.setDs();
        this.setDerivative();
    }
    Query.prototype.setDs = function () {
        if (this.dstime && this.ds) {
            this.downsample = this.dstime + '-' + this.ds;
        }
        else {
            this.downsample = '';
        }
    };
    Query.prototype.setDerivative = function () {
        var max = this.rateOptions.counterMax;
        this.rate = false;
        this.rateOptions = new RateOptions();
        switch (this.derivative) {
            case "rate":
                this.rate = true;
                break;
            case "counter":
                this.rate = true;
                this.rateOptions.counter = true;
                this.rateOptions.counterMax = max;
                this.rateOptions.resetValue = 1;
                break;
            case "gauge":
                this.rate = false;
                break;
        }
    };
    return Query;
})();
var Request = (function () {
    function Request() {
        this.start = '1h-ago';
        this.queries = [];
    }
    Request.prototype.prune = function () {
        var _this = this;
        for (var i = 0; i < this.queries.length; i++) {
            angular.forEach(this.queries[i], function (v, k) {
                var qi = _this.queries[i];
                switch (typeof v) {
                    case "string":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "boolean":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "object":
                        if (Object.keys(v).length == 0) {
                            delete qi[k];
                        }
                        break;
                }
            });
        }
    };
    return Request;
})();
var graphRefresh;
bosunControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', '$timeout', function ($scope, $http, $location, $route, $timeout) {
    $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
    $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
    $scope.rate_options = ["auto", "gauge", "counter", "rate"];
    var search = $location.search();
    var j = search.json;
    if (search.b64) {
        j = atob(search.b64);
    }
    var request = j ? JSON.parse(j) : new Request;
    $scope.index = parseInt($location.hash()) || 0;
    $scope.tagvs = [];
    $scope.sorted_tagks = [];
    $scope.query_p = [];
    angular.forEach(request.queries, function (q, i) {
        $scope.query_p[i] = new Query(q);
    });
    $scope.start = request.start;
    $scope.end = request.end;
    $scope.autods = search.autods != 'false';
    $scope.refresh = search.refresh == 'true';
    var duration_map = {
        "s": "s",
        "m": "m",
        "h": "h",
        "d": "d",
        "w": "w",
        "n": "M",
        "y": "y"
    };
    var isRel = /^(\d+)(\w)-ago$/;
    function RelToAbs(m) {
        return moment().utc().subtract(parseFloat(m[1]), duration_map[m[2]]).format();
    }
    function AbsToRel(s) {
        //Not strict parsing of the time format. For example, just "2014" will be valid
        var t = moment.utc(s, moment.defaultFormat).fromNow();
        return t;
    }
    function SwapTime(s) {
        if (!s) {
            return moment().utc().format();
        }
        var m = isRel.exec(s);
        if (m) {
            return RelToAbs(m);
        }
        return AbsToRel(s);
    }
    $scope.SwitchTimes = function () {
        $scope.start = SwapTime($scope.start);
        $scope.end = SwapTime($scope.end);
    };
    $scope.AddTab = function () {
        $scope.index = $scope.query_p.length;
        $scope.query_p.push(new Query);
    };
    $scope.setIndex = function (i) {
        $scope.index = i;
    };
    $scope.GetTagKByMetric = function (index) {
        $scope.tagvs[index] = new TagV;
        if (!$scope.query_p[index].metric) {
            return;
        }
        $http.get('/api/tagk/' + $scope.query_p[index].metric).success(function (data) {
            var q = $scope.query_p[index];
            var tags = new TagSet;
            q.metric_tags = {};
            for (var i = 0; i < data.length; i++) {
                var d = data[i];
                q.metric_tags[d] = true;
                if (q.tags) {
                    tags[d] = q.tags[d];
                }
                if (!tags[d]) {
                    tags[d] = '';
                }
                GetTagVs(d, index);
            }
            angular.forEach(q.tags, function (val, key) {
                if (val) {
                    tags[key] = val;
                }
            });
            q.tags = tags;
            // Make sure host is always the first tag.
            $scope.sorted_tagks[index] = Object.keys(tags);
            $scope.sorted_tagks[index].sort(function (a, b) {
                if (a == 'host') {
                    return -1;
                }
                else if (b == 'host') {
                    return 1;
                }
                return a.localeCompare(b);
            });
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
    };
    if ($scope.query_p.length == 0) {
        $scope.AddTab();
    }
    $http.get('/api/metric').success(function (data) {
        $scope.metrics = data;
    }).error(function (error) {
        $scope.error = 'Unable to fetch metrics: ' + error;
    });
    function GetTagVs(k, index) {
        $http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric).success(function (data) {
            data.sort();
            $scope.tagvs[index][k] = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
    }
    function getRequest() {
        request = new Request;
        request.start = $scope.start;
        request.end = $scope.end;
        angular.forEach($scope.query_p, function (p) {
            if (!p.metric) {
                return;
            }
            var q = new Query(p);
            var tags = q.tags;
            q.tags = new TagSet;
            angular.forEach(tags, function (v, k) {
                if (v && k) {
                    q.tags[k] = v;
                }
            });
            request.queries.push(q);
        });
        return request;
    }
    $scope.Query = function () {
        var r = getRequest();
        angular.forEach($scope.query_p, function (q, index) {
            var m = q.metric_tags;
            if (!m) {
                return;
            }
            angular.forEach(q.tags, function (key, tag) {
                if (m[tag]) {
                    return;
                }
                delete r.queries[index].tags[tag];
            });
        });
        r.prune();
        $location.search('b64', btoa(JSON.stringify(r)));
        $location.search('autods', $scope.autods ? undefined : 'false');
        $location.search('refresh', $scope.refresh ? 'true' : undefined);
        $route.reload();
    };
    request = getRequest();
    if (!request.queries.length) {
        return;
    }
    var autods = $scope.autods ? '&autods=' + $('#chart').width() : '';
    function get(noRunning) {
        $timeout.cancel(graphRefresh);
        if (!noRunning) {
            $scope.running = 'Running';
        }
        var autorate = '';
        for (var i = 0; i < request.queries.length; i++) {
            if (request.queries[i].derivative == 'auto') {
                autorate += '&autorate=' + i;
            }
        }
        $scope.animate();
        $http.get('/api/graph?' + 'b64=' + encodeURIComponent(btoa(JSON.stringify(request))) + autods + autorate).success(function (data) {
            $scope.result = data.Series;
            if (!$scope.result) {
                $scope.warning = 'No Results';
            }
            else {
                $scope.warning = '';
            }
            $scope.queries = data.Queries;
            $scope.running = '';
            $scope.error = '';
            var u = $location.absUrl();
            u = u.substr(0, u.indexOf('?')) + '?';
            u += 'b64=' + search.b64 + autods + autorate;
            $scope.url = u;
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        }).finally(function () {
            $scope.stop();
            if ($scope.refresh) {
                graphRefresh = $timeout(function () {
                    get(true);
                }, 5000);
            }
            ;
        });
    }
    ;
    get(false);
}]);
bosunApp.directive('tsPopup', function () {
    return {
        restrict: 'E',
        scope: {
            url: '='
        },
        template: '<button class="btn btn-default" data-html="true" data-placement="bottom">embed</button>',
        link: function (scope, elem, attrs) {
            var button = $('button', elem);
            scope.$watch(attrs.url, function (url) {
                if (!url) {
                    return;
                }
                var text = '<input type="text" onClick="this.select();" readonly="readonly" value="&lt;a href=&quot;' + url + '&quot;&gt;&lt;img src=&quot;' + url + '&.png=png&quot;&gt;&lt;/a&gt;">';
                button.popover({
                    content: text
                });
            });
        }
    };
});
bosunApp.directive('tsAlertHistory', function () {
    return {
        templateUrl: '/partials/alerthistory.html'
    };
});
bosunControllers.controller('HistoryCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    var keys = {};
    if (angular.isArray(search.key)) {
        angular.forEach(search.key, function (v) {
            keys[v] = true;
        });
    }
    else {
        keys[search.key] = true;
    }
    var params = Object.keys(keys).map(function (v) {
        return 'key=' + encodeURIComponent(v);
    }).join('&');
    $http.get('/api/alerts/details?' + params).success(function (data) {
        var selected_alerts = {};
        angular.forEach(data, function (v, ak) {
            if (!keys[ak]) {
                return;
            }
            v.History.map(function (h) {
                h.Time = moment.utc(h.Time);
            });
            angular.forEach(v.History, function (h, i) {
                if (i + 1 < v.History.length) {
                    h.EndTime = v.History[i + 1].Time;
                }
                else {
                    h.EndTime = moment.utc();
                }
            });
            selected_alerts[ak] = {
                History: v.History.reverse()
            };
        });
        if (Object.keys(selected_alerts).length > 0) {
            $scope.alert_history = selected_alerts;
        }
        else {
            $scope.error = 'No Matching Alerts Found';
        }
    }).error(function (err) {
        $scope.error = err;
    });
}]);
bosunControllers.controller('HostCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    $scope.host = search.host;
    $scope.time = search.time;
    $scope.tab = search.tab || "stats";
    $scope.idata = [];
    $scope.fsdata = [];
    $scope.metrics = [];
    $scope.mlink = function (m) {
        var r = new Request();
        var q = new Query();
        q.metric = m;
        q.tags = { 'host': $scope.host };
        r.queries.push(q);
        return r;
    };
    $scope.setTab = function (t) {
        $location.search('tab', t);
        $scope.tab = t;
    };
    $http.get('/api/metric/host/' + $scope.host).success(function (data) {
        $scope.metrics = data || [];
    });
    $http.get('/api/metadata/get?tagk=host&tagv=' + encodeURIComponent($scope.host)).success(function (data) {
        $scope.metadata = data;
    });
    var autods = '&autods=100';
    var cpu_r = new Request();
    cpu_r.start = $scope.time;
    cpu_r.queries = [
        new Query({
            metric: 'os.cpu',
            derivative: 'counter',
            tags: { host: $scope.host }
        })
    ];
    $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + autods).success(function (data) {
        if (!data.Series) {
            return;
        }
        data.Series[0].Name = 'Percent Used';
        $scope.cpu = data.Series;
    });
    var mem_r = new Request();
    mem_r.start = $scope.time;
    mem_r.queries.push(new Query({
        metric: "os.mem.total",
        tags: { host: $scope.host }
    }));
    mem_r.queries.push(new Query({
        metric: "os.mem.used",
        tags: { host: $scope.host }
    }));
    $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + autods).success(function (data) {
        if (!data.Series) {
            return;
        }
        data.Series[1].Name = "Used";
        $scope.mem_total = Math.max.apply(null, data.Series[0].Data.map(function (d) {
            return d[1];
        }));
        $scope.mem = [data.Series[1]];
    });
    $http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host).success(function (data) {
        $scope.interfaces = data;
        angular.forEach($scope.interfaces, function (i, idx) {
            $scope.idata[idx] = {
                Name: i
            };
            var net_bytes_r = new Request();
            net_bytes_r.start = $scope.time;
            net_bytes_r.queries = [
                new Query({
                    metric: "os.net.bytes",
                    rate: true,
                    rateOptions: { counter: true, resetValue: 1 },
                    tags: { host: $scope.host, iface: i, direction: "*" }
                })
            ];
            $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + autods).success(function (data) {
                if (!data.Series) {
                    return;
                }
                angular.forEach(data.Series, function (d) {
                    d.Data = d.Data.map(function (dp) {
                        return [dp[0], dp[1] * 8];
                    });
                    if (d.Name.indexOf("direction=out") != -1) {
                        d.Data = d.Data.map(function (dp) {
                            return [dp[0], dp[1] * -1];
                        });
                        d.Name = "out";
                    }
                    else {
                        d.Name = "in";
                    }
                });
                $scope.idata[idx].Data = data.Series;
            });
        });
    });
    $http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host).success(function (data) {
        angular.forEach(data, function (i, idx) {
            if (i == '/dev/shm') {
                return;
            }
            var fs_r = new Request();
            fs_r.start = $scope.time;
            fs_r.queries.push(new Query({
                metric: "os.disk.fs.space_total",
                tags: { host: $scope.host, disk: i }
            }));
            fs_r.queries.push(new Query({
                metric: "os.disk.fs.space_used",
                tags: { host: $scope.host, disk: i }
            }));
            $scope.fsdata[idx] = {
                Name: i
            };
            $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + autods).success(function (data) {
                if (!data.Series) {
                    return;
                }
                data.Series[1].Name = 'Used';
                var total = Math.max.apply(null, data.Series[0].Data.map(function (d) {
                    return d[1];
                }));
                var c_val = data.Series[1].Data.slice(-1)[0][1];
                var percent_used = c_val / total * 100;
                $scope.fsdata[idx].total = total;
                $scope.fsdata[idx].c_val = c_val;
                $scope.fsdata[idx].percent_used = percent_used;
                $scope.fsdata[idx].Data = [data.Series[1]];
            });
        });
    });
}]);
bosunControllers.controller('ItemsCtrl', ['$scope', '$http', function ($scope, $http) {
    $http.get('/api/metric').success(function (data) {
        $scope.metrics = data;
    }).error(function (error) {
        $scope.status = 'Unable to fetch metrics: ' + error;
    });
    $http.get('/api/tagv/host').success(function (data) {
        $scope.hosts = data;
    }).error(function (error) {
        $scope.status = 'Unable to fetch hosts: ' + error;
    });
}]);
var Tag = (function () {
    function Tag() {
    }
    return Tag;
})();
var DP = (function () {
    function DP() {
    }
    return DP;
})();
bosunControllers.controller('PutCtrl', ['$scope', '$http', '$route', function ($scope, $http, $route) {
    $scope.tags = [new Tag];
    var dp = new DP;
    dp.k = moment().utc().format(timeFormat);
    $scope.dps = [dp];
    $http.get('/api/metric').success(function (data) {
        $scope.metrics = data;
    }).error(function (error) {
        $scope.error = 'Unable to fetch metrics: ' + error;
    });
    $scope.Submit = function () {
        var data = [];
        var tags = {};
        angular.forEach($scope.tags, function (v, k) {
            if (v.k || v.v) {
                tags[v.k] = v.v;
            }
        });
        angular.forEach($scope.dps, function (v, k) {
            if (v.k && v.v) {
                var ts = parseInt(moment.utc(v.k, timeFormat).format('X'));
                data.push({
                    metric: $scope.metric,
                    timestamp: ts,
                    value: parseFloat(v.v),
                    tags: tags
                });
            }
        });
        $scope.running = 'submitting data...';
        $scope.success = '';
        $scope.error = '';
        $http.post('/api/put', data).success(function () {
            $scope.running = '';
            $scope.success = 'Data Submitted';
        }).error(function (error) {
            $scope.running = '';
            $scope.error = error.error.message;
        });
    };
    $scope.AddTag = function () {
        var last = $scope.tags[$scope.tags.length - 1];
        if (last.k && last.v) {
            $scope.tags.push(new Tag);
        }
    };
    $scope.AddDP = function () {
        var last = $scope.dps[$scope.dps.length - 1];
        if (last.k && last.v) {
            var dp = new DP;
            dp.k = moment.utc(last.k, timeFormat).add('seconds', 15).format(timeFormat);
            $scope.dps.push(dp);
        }
    };
    $scope.GetTagKByMetric = function () {
        $http.get('/api/tagk/' + $scope.metric).success(function (data) {
            if (!angular.isArray(data)) {
                return;
            }
            $scope.tags = [];
            for (var i = 0; i < data.length; i++) {
                var t = new Tag;
                t.k = data[i];
                $scope.tags.push(t);
            }
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });
    };
}]);
bosunControllers.controller('RuleCtrl', ['$scope', '$http', '$location', '$route', '$sce', function ($scope, $http, $location, $route, $sce) {
    var search = $location.search();
    var current_alert = atob(search.alert || '');
    var current_template = search.template;
    var expr = atob(search.expr || '') || 'avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", "")) > 10';
    var status_map = {
        "normal": 0,
        "warning": 1,
        "critical": 2
    };
    $scope.email = search.email || '';
    $scope.fromDate = search.fromDate || '';
    $scope.fromTime = search.fromTime || '';
    $scope.toDate = search.toDate || '';
    $scope.toTime = search.toTime || '';
    $scope.tab = search.tab || 'results';
    $scope.intervals = +search.intervals || 5;
    $scope.duration = +search.duration || null;
    if (!current_alert) {
        var alert_def = 'alert test {\n' + '	template = test\n' + '	crit = ' + expr + '\n' + '}';
        $location.search('alert', btoa(alert_def));
        $location.search('expr', null);
        return;
    }
    $scope.alert = current_alert;
    try {
        current_template = atob(current_template);
    }
    catch (e) {
        current_template = '';
    }
    if (!current_template) {
        var template_def = 'template test {\n' + '	subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}\n' + '	body = `<p>Name: {{.Alert.Name}}\n' + '	<p>Tags:\n' + '	<table>\n' + '		{{range $k, $v := .Group}}\n' + '			<tr><td>{{$k}}</td><td>{{$v}}</td></tr>\n' + '		{{end}}\n' + '	</table>`\n' + '}';
        $location.search('template', btoa(template_def));
        return;
    }
    $scope.template = current_template;
    $scope.shiftEnter = function ($event) {
        if ($event.keyCode == 13 && $event.shiftKey) {
            $scope.test();
        }
    };
    var alert_history = {};
    $scope.test = function () {
        $scope.error = '';
        $scope.warning = [];
        $location.search('alert', btoa($scope.alert));
        $location.search('template', btoa($scope.template));
        $location.search('fromDate', $scope.fromDate || null);
        $location.search('fromTime', $scope.fromTime || null);
        $location.search('toDate', $scope.toDate || null);
        $location.search('toTime', $scope.toTime || null);
        $location.search('tab', $scope.tab || 'results');
        $location.search('intervals', $scope.intervals || null);
        $location.search('duration', $scope.duration || null);
        $location.search('email', $scope.email || null);
        $scope.animate();
        var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
        var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
        if (!from.isValid()) {
            from = to;
        }
        if (!to.isValid()) {
            to = from;
        }
        if (!from.isValid() && !to.isValid()) {
            from = to = moment.utc();
        }
        var diff = from.diff(to);
        var intervals;
        if (diff == 0) {
            intervals = 1;
        }
        else if (Math.abs(diff) < 60 * 1000) {
            intervals = 2;
        }
        else {
            intervals = +($scope.intervals);
        }
        $scope.sets = [];
        function next(interval, first) {
            if (first === void 0) { first = false; }
            if (interval == 0 || $scope.stopped) {
                $scope.stop();
                $scope.remaining = 0;
                angular.forEach(alert_history, function (v) {
                    var h = v.History;
                    h.sort(function (a, b) {
                        return a.Time > b.Time;
                    });
                    angular.forEach(h, function (d, i) {
                        if (i + 1 < h.length) {
                            d.EndTime = h[i + 1].Time;
                        }
                        else {
                            d.EndTime = d.Time;
                        }
                    });
                    for (var i = 1; i < h.length; i++) {
                        if (h[i].Status == h[i - 1].Status) {
                            var r = h.splice(i, 1);
                            h[i - 1].EndTime = r[0].EndTime;
                            i--;
                        }
                    }
                });
                $scope.alert_history = alert_history;
                return;
            }
            $scope.remaining = interval;
            var date = from.format('YYYY-MM-DD');
            var time = from.format('HH:mm');
            var url = '/api/rule?' + 'alert=' + encodeURIComponent($scope.alert) + '&template=' + encodeURIComponent($scope.template) + '&date=' + encodeURIComponent(date) + '&time=' + encodeURIComponent(time) + '&email=' + encodeURIComponent($scope.email);
            var f = first ? '' : '&summary=true';
            $http.get(url + f).success(function (data) {
                var set = {
                    url: url,
                    time: moment.unix(data.Time).utc().format('YYYY-MM-DD HH:mm:ss'),
                    critical: data.Criticals.length,
                    warning: data.Warnings.length,
                    normal: data.Normals.length
                };
                procHistory(data);
                if (first) {
                    set.results = procResults(data);
                }
                $scope.sets.push(set);
                from.subtract(diff / (intervals - 1));
                next(interval - 1);
            }).error(function (error) {
                $scope.error = error;
                $scope.remaining = 0;
                $scope.stop();
            });
        }
        next(intervals, true);
    };
    function procHistory(data) {
        var t = moment.unix(data.Time).utc();
        function procStatus(st, d) {
            angular.forEach(d, function (v) {
                if (!alert_history[v]) {
                    alert_history[v] = { History: [] };
                }
                alert_history[v].History.push({
                    Time: t,
                    Status: st
                });
            });
        }
        procStatus('critical', data.Criticals);
        procStatus('warning', data.Warnings);
        procStatus('normal', data.Normals);
    }
    function procResults(data) {
        $scope.subject = data.Subject;
        $scope.body = $sce.trustAsHtml(data.Body);
        $scope.data = JSON.stringify(data.Data, null, '  ');
        angular.forEach(data.Warning, function (v) {
            $scope.warning.push(v);
        });
        var results = [];
        angular.forEach(data.Result, function (v, k) {
            results.push({
                group: k,
                result: v
            });
        });
        results.sort(function (a, b) {
            return status_map[b.result.Status] - status_map[a.result.Status];
        });
        return results;
    }
    $scope.show = function (set) {
        set.show = 'loading...';
        $scope.animate();
        $http.get(set.url).success(function (data) {
            set.results = procResults(data);
        }).error(function (error) {
            $scope.error = error;
        }).finally(function () {
            $scope.stop();
            delete (set.show);
        });
    };
    $scope.zws = function (v) {
        return v.replace(/([,{}()])/g, '$1\u200b');
    };
    $scope.scroll = function (id) {
        document.getElementById('time-' + id).scrollIntoView();
        $scope.show($scope.sets[id]);
    };
    $scope.setInterval = function () {
        var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
        var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
        if (!from.isValid() || !to.isValid()) {
            return;
        }
        var diff = from.diff(to);
        if (!diff) {
            return;
        }
        var intervals = +$scope.intervals;
        if (intervals < 2) {
            return;
        }
        diff /= 1000 * 60;
        var d = Math.abs(Math.round(diff / intervals));
        if (d < 1) {
            d = 1;
        }
        $scope.duration = d;
    };
    $scope.setDuration = function () {
        var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
        var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
        if (!from.isValid() || !to.isValid()) {
            return;
        }
        var diff = from.diff(to);
        if (!diff) {
            return;
        }
        var duration = +$scope.duration;
        if (duration < 1) {
            return;
        }
        $scope.intervals = Math.abs(Math.round(diff / duration / 1000 / 60));
    };
    $scope.halt = function () {
        $scope.stopped = true;
    };
    $scope.setInterval();
    $http.get('/api/templates').success(function (data) {
        $scope.alerts = data.Alerts;
        $scope.templates = data.Templates;
    });
    $scope.test();
}]);
bosunControllers.controller('SilenceCtrl', ['$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
    var search = $location.search();
    $scope.start = search.start;
    $scope.end = search.end;
    $scope.duration = search.duration;
    $scope.alert = search.alert;
    $scope.hosts = search.hosts;
    $scope.tags = search.tags;
    $scope.edit = search.edit;
    function get() {
        $http.get('/api/silence/get').success(function (data) {
            $scope.silences = data;
        }).error(function (error) {
            $scope.error = error;
        });
    }
    get();
    function getData() {
        var tags = ($scope.tags || '').split(',');
        if ($scope.hosts) {
            tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
        }
        tags = tags.filter(function (v) {
            return v != "";
        });
        var data = {
            start: $scope.start,
            end: $scope.end,
            duration: $scope.duration,
            alert: $scope.alert,
            tags: tags.join(','),
            edit: $scope.edit
        };
        return data;
    }
    var any = search.start || search.end || search.duration || search.alert || search.hosts || search.tags;
    var state = getData();
    $scope.change = function () {
        $scope.disableConfirm = true;
    };
    if (any) {
        $scope.error = null;
        $http.post('/api/silence/set', state).success(function (data) {
            if (data.length == 0) {
                data = [{ Name: '(none)' }];
            }
            $scope.testSilences = data;
        }).error(function (error) {
            $scope.error = error;
        });
    }
    $scope.test = function () {
        $location.search('start', $scope.start || null);
        $location.search('end', $scope.end || null);
        $location.search('duration', $scope.duration || null);
        $location.search('alert', $scope.alert || null);
        $location.search('hosts', $scope.hosts || null);
        $location.search('tags', $scope.tags || null);
        $route.reload();
    };
    $scope.confirm = function () {
        $scope.error = null;
        $scope.testSilences = null;
        state.confirm = "true";
        $http.post('/api/silence/set', state).error(function (error) {
            $scope.error = error;
        }).finally(function () {
            $scope.testSilences = null;
            get();
        });
    };
    $scope.clear = function (id) {
        if (!window.confirm('Clear this silence?')) {
            return;
        }
        $scope.error = null;
        $http.post('/api/silence/clear', { id: id }).error(function (error) {
            $scope.error = error;
        }).finally(function () {
            get();
        });
    };
    $scope.time = function (v) {
        var m = moment(v).utc();
        return m.format(timeFormat);
    };
}]);
bosunApp.directive('tsAckGroup', function () {
    return {
        scope: {
            ack: '=',
            groups: '=tsAckGroup',
            schedule: '='
        },
        templateUrl: '/partials/ackgroup.html',
        link: function (scope, elem, attrs) {
            scope.canAckSelected = scope.ack == 'Needs Acknowledgement';
            scope.panelClass = scope.$parent.panelClass;
            scope.btoa = scope.$parent.btoa;
            scope.encode = scope.$parent.encode;
            scope.shown = {};
            scope.collapse = function (i) {
                scope.shown[i] = !scope.shown[i];
            };
            scope.click = function ($event, idx) {
                scope.collapse(idx);
                if ($event.shiftKey && scope.schedule.checkIdx != undefined) {
                    var checked = scope.groups[scope.schedule.checkIdx].checked;
                    var start = Math.min(idx, scope.schedule.checkIdx);
                    var end = Math.max(idx, scope.schedule.checkIdx);
                    for (var i = start; i <= end; i++) {
                        if (i == idx) {
                            continue;
                        }
                        scope.groups[i].checked = checked;
                    }
                }
                scope.schedule.checkIdx = idx;
                scope.update();
            };
            scope.select = function (checked) {
                for (var i = 0; i < scope.groups.length; i++) {
                    scope.groups[i].checked = checked;
                }
                scope.update();
            };
            scope.update = function () {
                scope.canCloseSelected = true;
                scope.canForgetSelected = true;
                scope.anySelected = false;
                for (var i = 0; i < scope.groups.length; i++) {
                    var g = scope.groups[i];
                    if (!g.checked) {
                        continue;
                    }
                    scope.anySelected = true;
                    if (g.Active && g.Status != 'unknown') {
                        scope.canCloseSelected = false;
                    }
                    if (g.Status != 'unknown') {
                        scope.canForgetSelected = false;
                    }
                }
            };
            scope.multiaction = function (type) {
                var url = '/action?type=' + type;
                angular.forEach(scope.groups, function (group) {
                    if (!group.checked) {
                        return;
                    }
                    if (group.AlertKey) {
                        url += '&key=' + encodeURIComponent(group.AlertKey);
                    }
                    angular.forEach(group.Children, function (child) {
                        url += '&key=' + encodeURIComponent(child.AlertKey);
                    });
                });
                return url;
            };
            scope.history = function () {
                var url = '/history?';
                angular.forEach(scope.groups, function (group) {
                    if (!group.checked) {
                        return;
                    }
                    if (group.AlertKey) {
                        url += '&key=' + encodeURIComponent(group.AlertKey);
                    }
                    angular.forEach(group.Children, function (child) {
                        url += '&key=' + encodeURIComponent(child.AlertKey);
                    });
                });
                return url;
            };
        }
    };
});
bosunApp.factory('status', ['$http', '$q', function ($http, $q) {
    var cache = {};
    return function (ak) {
        var q = $q.defer();
        if (cache[ak]) {
            q.resolve(cache[ak]);
        }
        else {
            $http.get('/api/status?ak=' + encodeURIComponent(ak)).success(function (data) {
                angular.forEach(data, function (v, k) {
                    v.Touched = moment(v.Touched).utc();
                    angular.forEach(v.History, function (v, k) {
                        v.Time = moment(v.Time).utc();
                    });
                    v.last = v.History[v.History.length - 1];
                    if (v.Actions && v.Actions.length > 0) {
                        v.LastAction = v.Actions[0];
                    }
                    cache[k] = v;
                });
                q.resolve(cache[ak]);
            }).error(q.reject);
        }
        return q.promise;
    };
}]);
bosunApp.directive('tsState', ['status', function ($status) {
    return {
        templateUrl: '/partials/alertstate.html',
        link: function (scope, elem, attrs) {
            scope.name = scope.child.AlertKey;
            scope.loading = true;
            $status(scope.child.AlertKey).then(function (st) {
                scope.state = st;
                scope.loading = false;
            }, function (err) {
                alert(err);
                scope.loading = false;
            });
            scope.action = function (type) {
                var key = encodeURIComponent(scope.name);
                return '/action?type=' + type + '&key=' + key;
            };
            scope.zws = function (v) {
                if (!v) {
                    return '';
                }
                return v.replace(/([,{}()])/g, '$1\u200b');
            };
        }
    };
}]);
bosunApp.directive('tsAck', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/ack.html'
    };
});
bosunApp.directive('tsClose', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/close.html'
    };
});
bosunApp.directive('tsForget', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/forget.html'
    };
});
